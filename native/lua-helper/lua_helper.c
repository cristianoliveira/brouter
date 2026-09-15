// brouter-lua-helper: isolated C Lua 5.4 route-script evaluator.
//
// TASK-0029 first slice. The helper is a standalone process so the Go
// router stays CGO-free and a runaway or native-faulting script cannot
// take the router down: the parent enforces a wall-clock kill and this
// process enforces an instruction budget plus a hard allocation cap.
//
// Protocol (byte-safe, no escaping anywhere — URLs are passed raw):
//   stdin:  [uint32 BE pair count]
//           { [uint32 BE key len][key][uint32 BE value len][value] }*
//   stdout: same framing with keys: status, target?, category?
//
// Recognized request keys: "script" (full Lua source), "epoch"
// (decimal seconds; the single injected clock sample), and
// "url.original" / "url.scheme" / "url.host" / "url.port" /
// "url.path" / "url.query" / "url.fragment".
//
// Response statuses: "ok" (+ "target"), "nil" (defer to static rules),
// "error" (+ "category"). Categories are fixed allowlisted strings
// with no script text, URL parts, or target names embedded.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "lua.h"
#include "lauxlib.h"
#include "lualib.h"

#define MEMORY_CAP_BYTES (8u << 20)  // 8 MiB hard allocator cap
#define INSTRUCTION_BUDGET 20000000u // measured in TASK-0028
#define MAX_TARGET_LEN 256

typedef struct {
	size_t used;
	long long epoch; // single sample injected by the parent
} MemState;

// Hard allocation cap: shrinking allocations must never wrap the check,
// so the delta is computed signed.
static void *limited_alloc(void *ud, void *ptr, size_t osize, size_t nsize) {
	MemState *mem = (MemState *)ud;
	long long delta = (long long)nsize - (long long)osize;
	if (nsize == 0) {
		free(ptr);
		if (delta < 0) mem->used = mem->used >= (size_t)(-delta) ? mem->used - (size_t)(-delta) : 0;
		return NULL;
	}
	if (delta > 0 && (long long)mem->used + delta > (long long)MEMORY_CAP_BYTES) {
		return NULL; // Lua converts this into a memory error
	}
	void *p = realloc(ptr, nsize);
	if (p != NULL) {
		if (delta >= 0) mem->used += (size_t)delta;
		else mem->used -= (size_t)(-delta);
	}
	return p;
}

static void timeout_hook(lua_State *L, lua_Debug *ar) {
	(void)ar;
	luaL_error(L, "script-timeout");
}

static MemState *memState = NULL;

static void push_utc(lua_State *L);

// The clock: one sample per invocation, injected by the parent, so
// script time is deterministic and host-free. Both epoch() and utc()
// read the same frozen sample.
static int utils_epoch(lua_State *L) {
	lua_pushinteger(L, (lua_Integer)memState->epoch);
	return 1;
}

static int utils_utc(lua_State *L) {
	push_utc(L);
	return 1;
}

static void push_utc(lua_State *L) {
	time_t t = (time_t)memState->epoch;
	struct tm tmv;
	gmtime_r(&t, &tmv);
	lua_createtable(L, 0, 6);
	lua_pushinteger(L, tmv.tm_year + 1900); lua_setfield(L, -2, "year");
	lua_pushinteger(L, tmv.tm_mon + 1);     lua_setfield(L, -2, "month");
	lua_pushinteger(L, tmv.tm_mday);        lua_setfield(L, -2, "day");
	lua_pushinteger(L, tmv.tm_hour);        lua_setfield(L, -2, "hour");
	lua_pushinteger(L, tmv.tm_min);         lua_setfield(L, -2, "min");
	lua_pushinteger(L, tmv.tm_sec);         lua_setfield(L, -2, "sec");
}

typedef struct {
	char *key;
	char *value;
} Pair;

// Reads exactly n bytes and NUL-terminates: every consumer uses
// strcmp/strlen on these buffers, so an unterminated read would let a
// value run into the next pair's bytes (a real past defect).
static char *read_exact(FILE *f, size_t n) {
	char *buf = malloc(n + 1);
	if (buf == NULL) return NULL;
	if (fread(buf, 1, n, f) != n) {
		free(buf);
		return NULL;
	}
	buf[n] = 0;
	return buf;
}

// Reads the framed request; returns the pair count or -1 on error.
static long read_request(FILE *f, Pair **out) {
	unsigned int count = 0;
	if (fread(&count, 4, 1, f) != 1 || count == 0 || count > 64) return -1;
	Pair *pairs = calloc(count, sizeof(Pair));
	if (pairs == NULL) return -1;
	for (unsigned int i = 0; i < count; i++) {
		unsigned int klen = 0, vlen = 0;
		if (fread(&klen, 4, 1, f) != 1 || klen == 0 || klen > 256) goto fail;
		char *key = read_exact(f, klen);
		if (key == NULL) goto fail;
		if (fread(&vlen, 4, 1, f) != 1 || vlen > (8u << 20)) {
			free(key);
			goto fail;
		}
		char *value = read_exact(f, vlen);
		if (value == NULL) {
			free(key);
			goto fail;
		}
		pairs[i].key = key;
		pairs[i].value = value;
	}
	*out = pairs;
	return (long)count;
fail:
	for (unsigned int j = 0; j < count; j++) {
		free(pairs[j].key);
		free(pairs[j].value);
	}
	free(pairs);
	return -1;
}

static const char *lookup(Pair *pairs, long n, const char *key) {
	for (long i = 0; i < n; i++) {
		if (strcmp(pairs[i].key, key) == 0) return pairs[i].value;
	}
	return NULL;
}

static void free_request(Pair *pairs, long n) {
	for (long i = 0; i < n; i++) {
		free(pairs[i].key);
		free(pairs[i].value);
	}
	free(pairs);
}

static void write_pair(FILE *f, const char *key, const char *value) {
	unsigned int klen = (unsigned int)strlen(key);
	unsigned int vlen = value ? (unsigned int)strlen(value) : 0;
	fwrite(&klen, 4, 1, f);
	fwrite(key, 1, klen, f);
	fwrite(&vlen, 4, 1, f);
	if (vlen) fwrite(value, 1, vlen, f);
}

// Maps a protected-call failure to a safe category: no script text, no
// URL parts, no target names. Memory errors are their own class; the
// timeout hook's sentinel identifies budget exhaustion.
static const char *classify_failure(lua_State *L, int rc) {
	if (rc == LUA_ERRMEM) return "memory-cap";
	const char *msg = lua_tostring(L, -1);
	if (msg && strstr(msg, "script-timeout") != NULL) return "script-timeout";
	// 5.4 raises allocator failures through the runtime path with a
	// "not enough memory" message; sniffing is diagnostic-only — a
	// script-authored error containing "memory" may land here instead
	// of script-error, which changes no safety property (both fall
	// back to static rules).
	if (msg && (strstr(msg, "memory") != NULL || strstr(msg, "not enough") != NULL)) return "memory-cap";
	return "script-error";
}

// Loads the script, defines ctx/utils, calls the global route(ctx),
// and classifies the outcome. Exactly one of status/target/category
// combinations is produced; target is set only when status is "ok"
// and is owned by the Lua state (valid until lua_close).
static void evaluate(lua_State *L, const char *script,
		const char **status, const char **category, const char **target) {
	// Chunkname is a fixed literal so script text can never leak into
	// error identities.
	if (luaL_loadbuffer(L, script, strlen(script), "route-script") != LUA_OK) {
		*category = "script-load-error";
		return;
	}
	if (lua_pcall(L, 0, 0, 0) != LUA_OK) {
		*category = classify_failure(L, -1);
		return;
	}
	if (lua_getglobal(L, "route") != LUA_TFUNCTION) {
		*category = "missing-route-function";
		return;
	}
	lua_getglobal(L, "ctx");
	if (lua_pcall(L, 1, 1, 0) != LUA_OK) {
		*category = classify_failure(L, -1);
		return;
	}
	if (lua_isnil(L, -1)) {
		*status = "nil";
		return;
	}
	// A non-string return is a script bug, same class as a runtime
	// error (contract: script-error, not a separate category).
	if (lua_type(L, -1) != LUA_TSTRING) {
		*category = "script-error";
		return;
	}
	size_t len = 0;
	const char *t = lua_tolstring(L, -1, &len);
	if (len > MAX_TARGET_LEN) {
		*category = "script-error";
		return;
	}
	char *copy = malloc(len + 1);
	if (copy == NULL) {
		*category = "memory-cap";
		return;
	}
	memcpy(copy, t, len);
	copy[len] = 0;
	*status = "ok";
	*target = copy;
}

static int readonly_error(lua_State *L) {
	return luaL_error(L, "ctx is read-only");
}

// Replaces the real table at the top of the stack with a read-only
// proxy over it: reads pass through to the real table, writes are
// denied, and the metatable is hidden so the real table cannot be
// reached through it. rawset and setmetatable are stripped globally,
// closing the two remaining mutation paths (a rawset on the proxy only
// shadows script-local reads; the real table stays unreachable).
static void make_readonly(lua_State *L) {
	lua_newtable(L); // P: the table scripts actually hold
	lua_createtable(L, 0, 3);
	lua_pushvalue(L, -3);
	lua_setfield(L, -2, "__index");
	lua_pushcfunction(L, readonly_error);
	lua_setfield(L, -2, "__newindex");
	lua_pushliteral(L, "protected");
	lua_setfield(L, -2, "__metatable");
	lua_setmetatable(L, -2);
	lua_replace(L, -2);
}

int main(void) {
	Pair *pairs = NULL;
	long n = read_request(stdin, &pairs);
	if (n < 0) return 2;

	const char *script = lookup(pairs, n, "script");
	if (script == NULL) {
		free_request(pairs, n);
		return 2;
	}
	const char *epochStr = lookup(pairs, n, "epoch");
	long long epoch = epochStr ? atoll(epochStr) : 0;

	MemState memStateStorage = {0, epoch};
	memState = &memStateStorage;
	lua_State *L = lua_newstate(limited_alloc, &memStateStorage);

	const char *status = "error";
	const char *category = NULL;
	const char *target = NULL;
	if (L == NULL) {
		category = "memory-cap";
	} else {
		// Instruction budget covers script load AND execution.
		lua_sethook(L, timeout_hook, LUA_MASKCOUNT, INSTRUCTION_BUDGET);

		// Deny-by-default libraries: base/table/string only. os, io,
		// the package loader, and the debug library are never opened,
		// and the globals are stripped again in case base grows one.
		luaL_requiref(L, "_G", luaopen_base, 1);
		luaL_requiref(L, LUA_TABLIBNAME, luaopen_table, 1);
		luaL_requiref(L, LUA_STRLIBNAME, luaopen_string, 1);
		lua_pop(L, 3);
		// print and warn are stripped too: the framed response is the only
	// thing this process may write to stdout, and a script that prints
	// would corrupt the framing. Scripts have no output channel.
	const char *banned[] = {"os", "io", "print", "warn", "dofile", "loadfile", "require", "load", "debug", "rawset", "setmetatable", NULL};
		for (int i = 0; banned[i]; i++) {
			lua_pushnil(L);
			lua_setglobal(L, banned[i]);
		}

		// utils: injected clock (single sample) + UTC decomposition,
		// both as functions over the same frozen sample.
		lua_createtable(L, 0, 2);
		lua_pushcclosure(L, utils_epoch, 0);
		lua_setfield(L, -2, "epoch");
		lua_pushcclosure(L, utils_utc, 0);
		lua_setfield(L, -2, "utc");
		make_readonly(L);
	lua_setglobal(L, "utils");

		// ctx = { url = {...} }: immutable contract fields only, raw
		// bytes, no decoding, no userinfo.
		const char *reqKeys[] = {"url.original", "url.scheme", "url.host",
			"url.port", "url.path", "url.query", "url.fragment"};
		const char *urlNames[] = {"original", "scheme", "host",
			"port", "path", "query", "fragment"};
		lua_createtable(L, 0, 7);
		for (int i = 0; i < 7; i++) {
			const char *v = lookup(pairs, n, reqKeys[i]);
			lua_pushstring(L, v ? v : "");
			lua_setfield(L, -2, urlNames[i]);
		}
		make_readonly(L);
		lua_createtable(L, 0, 1);
		lua_insert(L, -2);
		lua_setfield(L, -2, "url");
		make_readonly(L);
		lua_setglobal(L, "ctx");

		evaluate(L, script, &status, &category, &target);
	}

	unsigned int responsePairs = 1 + (category ? 1u : 0u) + (target ? 1u : 0u);
	fwrite(&responsePairs, 4, 1, stdout);
	write_pair(stdout, "status", status);
	if (category) write_pair(stdout, "category", category);
	if (target) write_pair(stdout, "target", target);
	free_request(pairs, n);
	if (L) lua_close(L);
	return 0;
}
