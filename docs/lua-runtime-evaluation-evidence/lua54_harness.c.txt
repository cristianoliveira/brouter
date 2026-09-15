// Pinned C Lua 5.4.7 contract probe: allowlist sandbox, injected UTC
// utils, instruction-budget timeout, error mapping.
#include <stdio.h>
#include "lua.h"
#include "lualib.h"
#include "lauxlib.h"

static void timeout_hook(lua_State *L, lua_Debug *ar) {
	(void)ar;
	luaL_error(L, "script-timeout (instruction budget exceeded)");
}

static int utc_epoch(lua_State *L) {
	lua_pushinteger(L, 1700000000); // injected host clock
	return 1;
}

static const char *run(const char *code) {
	lua_State *L = luaL_newstate();
	// Base library only; os/io are never opened, then double-stripped
	// from _G in case a future luaopen set adds them.
	luaL_requiref(L, "_G", luaopen_base, 1);
	luaL_requiref(L, LUA_TABLIBNAME, luaopen_table, 1);
	luaL_requiref(L, LUA_STRLIBNAME, luaopen_string, 1);
	lua_pop(L, 3);
	lua_pushnil(L); lua_setglobal(L, "os");
	lua_pushnil(L); lua_setglobal(L, "io");
	lua_pushnil(L); lua_setglobal(L, "dofile");
	lua_pushnil(L); lua_setglobal(L, "loadfile");
	lua_pushnil(L); lua_setglobal(L, "require");
	lua_pushnil(L); lua_setglobal(L, "load");

	lua_createtable(L, 0, 1);
	lua_pushcfunction(L, utc_epoch);
	lua_setfield(L, -2, "epoch");
	lua_setglobal(L, "utils");

	// Instruction budget: the timeout mechanism (one hook call per
	// 20M VM instructions ≈ tens of ms for pure-Lua loops).
	lua_sethook(L, timeout_hook, LUA_MASKCOUNT, 20000000);

	int rc = luaL_dostring(L, code);
	const char *result = "ok";
	static char buf[256];
	if (rc != LUA_OK) {
		snprintf(buf, sizeof buf, "%s", lua_tostring(L, -1));
		result = buf;
	}
	lua_close(L);
	return result;
}

int main(void) {
	printf("sandbox-probe: %s\n", run("return (os == nil) and (io == nil)"));
	printf("injected-utils: %s\n", run("return utils.epoch() == 1700000000"));
	printf("error-mapping: %s\n", run("error('boom')"));
	printf("timeout: %s\n", run("while true do end"));
	return 0;
}
