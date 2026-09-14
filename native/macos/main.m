#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>
#import <spawn.h>

extern char **environ;


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
	// Same contract as the Go resolver (internal/infra/config): absolute
	// $XDG_CONFIG_HOME wins; otherwise $HOME/.config — on macOS too, so
	// GUI launches without shell startup environment resolve the same
	// location as the CLI. There is no legacy Application Support
	// fallback. Returns nil when no root can be determined; callers
	// must surface that visibly.
	NSString *xdg = [[[NSProcessInfo processInfo] environment] objectForKey:@"XDG_CONFIG_HOME"];
	// Contract is $HOME, so read the environment variable — not
	// NSHomeDirectory, which reports the passwd home and would ignore
	// the very variable the resolver contract names.
	NSString *home = [[[NSProcessInfo processInfo] environment] objectForKey:@"HOME"];
	if (xdg.length > 0 && [xdg hasPrefix:@"/"]) {
		return [xdg stringByAppendingPathComponent:@"brouter/config.toml"];
	}
	if (home.length == 0) {
		return nil;
	}
	return [[home stringByAppendingPathComponent:@".config"]
		stringByAppendingPathComponent:@"brouter/config.toml"];
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

	if (config == nil) {
		AppendLog(@"error: cannot determine the user config directory: "
			@"set $XDG_CONFIG_HOME to an absolute path, or set $HOME");
		return;
	}

	if (![[NSFileManager defaultManager] isExecutableFileAtPath:brouter]) {
		AppendLog([NSString stringWithFormat:@"error: embedded brouter missing at %@", brouter]);
		return;
	}

	for (NSString *url in urls) {
		// Privacy: log the event, never the URL. Query strings and
		// fragments can carry secrets; the browser's own history and
		// the destination server are the record of what was opened.
		AppendLog(@"forwarding URL event");

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
			AppendLog([NSString stringWithFormat:@"error: spawn failed (%d)", spawnErr]);
		}
	}
}

// PresenceController owns the single menu-bar status item. It lives on
// the same NSApplication loop that delivers URL events; it is created
// once per app instance, so repeated OS opens never add items. The
// menu identifies the app as running — nothing more: health, routing
// success, and default-browser status are deliberately not claimed,
// and no polling runs behind this slice.
@interface PresenceController : NSObject
- (void)install;
- (NSMenu *)makeMenu;
- (void)quit:(id)sender;
@end

@implementation PresenceController {
	// Retained for the app lifetime: an unretained NSStatusItem is
	// deallocated and its menu-bar icon vanishes.
	NSStatusItem *_statusItem;
}

// Builds the presence menu. Extracted so the native self-test harness
// can assert the Quit wiring exactly as AppKit dispatches it
// (sendAction:to:from:) without a status bar.
- (NSMenu *)makeMenu {
	NSMenu *menu = [[NSMenu alloc] init];
	NSMenuItem *running = [[NSMenuItem alloc]
		initWithTitle:@"Brouter — running" action:nil keyEquivalent:@""];
	running.enabled = NO;
	[menu addItem:running];
	[menu addItem:[NSMenuItem separatorItem]];
	// The item targets self and quit:, which forwards to NSApplication
	// terminate: — the action selector MUST exist on the target, or a
	// click raises an unrecognized-selector exception. terminate: ends
	// this handler process only. It never touches browsers, the config
	// file, or OS defaults. Quit is not a persistent disable switch: a
	// later OS URL delivery may relaunch the selected handler.
	NSMenuItem *quit = [[NSMenuItem alloc]
		initWithTitle:@"Quit Brouter" action:@selector(quit:) keyEquivalent:@"q"];
	quit.target = self;
	[menu addItem:quit];
	return menu;
}

- (void)install {
	_statusItem = [[NSStatusBar systemStatusBar]
		statusItemWithLength:NSVariableStatusItemLength];

	// A system symbol renders as a template image: AppKit recolors it
	// for light and dark menu bars automatically.
	NSImage *icon = [NSImage imageWithSystemSymbolName:@"arrow.triangle.branch"
		accessibilityDescription:@"Brouter"];
	if (icon == nil) {
		AppendLog(@"error: status item symbol unavailable; menu bar presence incomplete");
	} else {
		_statusItem.button.image = icon;
	}
	_statusItem.button.accessibilityLabel = @"Brouter";
	_statusItem.button.toolTip = @"Brouter — running";
	_statusItem.menu = [self makeMenu];

	AppendLog(@"menu bar presence installed");
}

- (void)quit:(id)sender {
	[[NSApplication sharedApplication] terminate:self];
}

@end

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

		// Launched as a long-running handler. LaunchServices delivers the
		// GURL Apple Event only to an initialized NSApplication: a bare
		// NSRunLoop never services the event queue, and `open -a` fails
		// with the -1712 timeout (observed defect). NSApplication's run
		// loop registers with the Window Server and dispatches Apple
		// Events to the NSAppleEventManager handler.
		NSApplication *application = [NSApplication sharedApplication];
		EventRedirector *redirector = [[EventRedirector alloc] init];
		[[NSAppleEventManager sharedAppleEventManager]
			setEventHandler:redirector
				andSelector:@selector(handleGetURLEvent:withReplyEvent:)
				forEventClass:kInternetEventClass
				   andEventID:kAEGetURL];

		// Menu-bar presence rides the same run loop and application:
		// no daemon, no tray framework, no focus change, no Dock icon.
		PresenceController *presence = [[PresenceController alloc] init];
		[presence install];

		AppendLog(@"handler started; waiting for URL events");
		[application run];
		return 0;
	}
}
