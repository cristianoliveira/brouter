#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>
#import <spawn.h>

extern char **environ;

// The Internet get-URL event. Stable four-char codes; some SDK header
// configurations gate the canonical constants, so they are declared
// here rather than depended on.
enum { kInternetEventClass = 'GURL' };
enum { kAEGetURL = 'GURL' };

// BrouterHandler is a minimal macOS URL handler. macOS delivers http and
// https URLs to it (Apple Events when running, argv when launched) and
// the shim forwards every URL, as structured arguments, to the brouter
// binary embedded beside it. It contains no routing logic: what happens
// to the URL is entirely brouter's decision.

static NSString *HandlerLogPath(void) {
	NSString *override = [[[NSProcessInfo processInfo] environment] objectForKey:@"BRROUTER_HANDLER_LOG"];
	if (override.length > 0) {
		return override;
	}
	return [NSHomeDirectory() stringByAppendingPathComponent:@"Library/Logs/brouter-handler.log"];
}

static NSString *ConfigPath(void) {
	// Same location the brouter CLI resolves via os.UserConfigDir:
	// absolute, independent of working directory or PATH.
	return [NSHomeDirectory() stringByAppendingPathComponent:
		@"Library/Application Support/brouter/config.toml"];
}

static NSString *EmbeddedBrouterPath(void) {
	// The brouter binary lives beside the shim inside the bundle, so no
	// PATH lookup is ever involved.
	NSString *exe = [[[NSProcessInfo processInfo] arguments] firstObject];
	return [[exe stringByDeletingLastPathComponent] stringByAppendingPathComponent:@"brouter"];
}

static void AppendLog(NSString *message) {
	NSString *path = HandlerLogPath();
	NSString *line = [NSString stringWithFormat:@"%@ %@\n", [NSDate date], message];
	NSFileHandle *handle = [NSFileHandle fileHandleForWritingAtPath:path];
	if (handle == nil) {
		[[NSFileManager defaultManager] createFileAtPath:path contents:nil attributes:nil];
		handle = [NSFileHandle fileHandleForWritingAtPath:path];
	}
	if (handle == nil) {
		return; // diagnostics are best-effort; never crash the handler
	}
	[handle seekToEndOfFile];
	[handle writeData:[line dataUsingEncoding:NSUTF8StringEncoding]];
	[handle closeFile];
}

// Forwards each URL by spawning the embedded brouter with structured
// arguments — never a shell. The child inherits no terminal: its output
// is appended to the diagnostics log so GUI-originated failures stay
// visible. spawn is fire-and-forget; per-child exit status is not
// tracked (documented limitation, consistent with the launch probes).
static void ForwardURLs(NSArray<NSString *> *urls) {
	NSString *brouter = EmbeddedBrouterPath();
	NSString *config = ConfigPath();
	NSString *log = HandlerLogPath();

	if (![[NSFileManager defaultManager] isExecutableFileAtPath:brouter]) {
		AppendLog([NSString stringWithFormat:@"error: embedded brouter missing at %@", brouter]);
		return;
	}

	for (NSString *url in urls) {
		AppendLog([NSString stringWithFormat:@"forwarding %@", url]);

		posix_spawn_file_actions_t actions;
		posix_spawn_file_actions_init(&actions);
		posix_spawn_file_actions_addopen(&actions, STDOUT_FILENO, log.fileSystemRepresentation,
			O_WRONLY | O_APPEND | O_CREAT, 0644);
		posix_spawn_file_actions_adddup2(&actions, STDOUT_FILENO, STDERR_FILENO);

		char *argv[] = {
			(char *)brouter.fileSystemRepresentation,
			(char *)"open",
			(char *)"--config",
			(char *)config.fileSystemRepresentation,
			(char *)url.UTF8String,
			NULL,
		};
		pid_t pid = -1;
		int spawnErr = posix_spawn(&pid, brouter.fileSystemRepresentation, &actions, NULL, argv, environ);
		posix_spawn_file_actions_destroy(&actions);
		if (spawnErr != 0) {
			AppendLog([NSString stringWithFormat:@"error: spawn failed (%d) for %@", spawnErr, url]);
		}
	}
}

@interface EventRedirector : NSObject
- (void)handleGetURLEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply;
@end

@implementation EventRedirector
- (void)handleGetURLEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply {
	NSString *url = [[event descriptorForKeyword:keyDirectObject] stringValue];
	if (url.length > 0) {
		ForwardURLs(@[url]);
	}
}
@end

int main(int argc, const char *argv[]) {
	@autoreleasepool {
		NSMutableArray<NSString *> *argvURLs = [NSMutableArray array];
		for (NSInteger i = 1; i < argc; i++) {
			NSString *argument = [NSString stringWithUTF8String:argv[i]];
			if ([argument hasPrefix:@"http://"] || [argument hasPrefix:@"https://"]) {
				[argvURLs addObject:argument];
			}
		}

		if (argvURLs.count > 0) {
			// Launched with URLs on the command line (or via LaunchServices
			// argv handoff): forward and exit. Testable without LaunchServices.
			ForwardURLs(argvURLs);
			return 0;
		}

		// Launched as a long-running handler: register for Apple Events and
		// keep the run loop alive so macOS can deliver URLs while open.
		EventRedirector *redirector = [[EventRedirector alloc] init];
		[[NSAppleEventManager sharedAppleEventManager]
			setEventHandler:redirector
				andSelector:@selector(handleGetURLEvent:withReplyEvent:)
				forEventClass:kInternetEventClass
				   andEventID:kAEGetURL];

		AppendLog(@"handler started; waiting for URL events");
		[[NSRunLoop mainRunLoop] run];
		return 0;
	}
}
