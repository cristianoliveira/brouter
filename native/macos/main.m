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

#pragma mark - Recent Activity (TASK-0023)

// ActivityRecord is a structured, privacy-safe diagnostic entry. Every
// field comes from a fixed allowlist or a number: there is no field a
// URL, path, profile, or child stderr could reach. Rendering can
// therefore never leak query strings, fragments, credentials, hosts,
// or filesystem paths by construction.
@interface ActivityRecord : NSObject
- (instancetype)initWithTimestamp:(NSDate *)timestamp
	kind:(NSString *)kind
	entryPoint:(NSString *)entryPoint
	requestID:(NSUInteger)requestID
	errorCategory:(NSString *)errorCategory
	detail:(NSInteger)detail;
@property (readonly) NSDate *timestamp;
@property (readonly) NSString *kind;          // receipt | preflight | dispatch-started | dispatch-failed
@property (readonly) NSString *entryPoint;    // os-event | argv
@property (readonly) NSUInteger requestID;    // session-local monotonic
@property (readonly, nullable) NSString *errorCategory; // allowlisted
@property (readonly) NSInteger detail;        // errno or -1
@end

@implementation ActivityRecord
- (instancetype)initWithTimestamp:(NSDate *)timestamp
	kind:(NSString *)kind
	entryPoint:(NSString *)entryPoint
	requestID:(NSUInteger)requestID
	errorCategory:(NSString *)errorCategory
	detail:(NSInteger)detail {
	if (self = [super init]) {
		_timestamp = timestamp;
		_kind = kind;
		_entryPoint = entryPoint;
		_requestID = requestID;
		_errorCategory = errorCategory;
		_detail = detail;
	}
	return self;
}
@end

// RecentActivity is the bounded in-memory store behind the menu.
// Capacity is fixed (20); eviction drops the oldest entries first.
// Clock and request-ID sources are injectable so tests pin time and
// ordering deterministically. Records live on the main thread only —
// the same thread that receives URL events and opens the menu — and
// are never persisted.
@interface RecentActivity : NSObject
- (instancetype)initWithCapacity:(NSUInteger)capacity;
- (void)recordKind:(NSString *)kind
	entryPoint:(NSString *)entryPoint
	errorCategory:(NSString *)errorCategory
	detail:(NSInteger)detail;
- (NSArray<ActivityRecord *> *)entries; // newest first
- (void)clear;                          // memory only; never touches files
- (NSString *)displayLineForEntry:(ActivityRecord *)entry;
@property (copy) NSDate *(^now)(void);              // injectable clock
@property (copy) NSUInteger (^nextRequestID)(void); // injectable IDs
@end

@implementation RecentActivity {
	NSMutableArray<ActivityRecord *> *_entries;
	NSUInteger _capacity;
}

- (instancetype)initWithCapacity:(NSUInteger)capacity {
	if (self = [super init]) {
		_entries = [NSMutableArray array];
		_capacity = capacity;
		_now = ^{ return [NSDate date]; };
		__block NSUInteger counter = 0;
		_nextRequestID = ^{ return ++counter; };
	}
	return self;
}

- (instancetype)init {
	return [self initWithCapacity:20];
}

- (void)recordKind:(NSString *)kind
	entryPoint:(NSString *)entryPoint
	errorCategory:(NSString *)errorCategory
	detail:(NSInteger)detail {
	ActivityRecord *record = [[ActivityRecord alloc]
		initWithTimestamp:self.now()
		kind:kind
		entryPoint:entryPoint
		requestID:self.nextRequestID()
		errorCategory:errorCategory
		detail:detail];
	[_entries insertObject:record atIndex:0];
	while (_entries.count > _capacity) {
		[_entries removeLastObject]; // deterministic oldest-first eviction
	}
}

- (NSArray<ActivityRecord *> *)entries {
	return [_entries copy];
}

- (void)clear {
	[_entries removeAllObjects];
}

- (NSString *)displayLineForEntry:(ActivityRecord *)entry {
	static NSDateFormatter *formatter = nil;
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		formatter = [[NSDateFormatter alloc] init];
		formatter.dateFormat = @"HH:mm:ss";
		formatter.timeZone = [NSTimeZone timeZoneForSecondsFromGMT:0];
	});
	NSString *kindText = entry.kind;
	if ([entry.kind isEqualToString:@"dispatch-started"]) {
		// A spawned child is not proof of a browser or a loaded page.
		kindText = @"dispatch started, outcome unknown";
	} else if ([entry.kind isEqualToString:@"dispatch-failed"]) {
		kindText = @"dispatch failed";
	} else if (entry.errorCategory != nil) {
		kindText = @"preflight failed";
	}
	NSMutableString *line = [NSMutableString stringWithFormat:@"%@ #%lu %@ %@",
		[formatter stringFromDate:entry.timestamp],
		(unsigned long)entry.requestID, entry.entryPoint, kindText];
	if (entry.errorCategory != nil) {
		[line appendFormat:@": %@", entry.errorCategory];
	}
	if (entry.detail >= 0) {
		[line appendFormat:@" (errno %ld)", (long)entry.detail];
	}
	return line;
}
@end

// Forwards each URL by spawning the embedded brouter with structured
// arguments — never a shell. The child inherits no terminal: its output
// is appended to the diagnostics log so GUI-originated failures stay
// visible. spawn is fire-and-forget; per-child exit status is not
// tracked (documented limitation, consistent with the launch probes).
//
// Each step records a structured activity entry: receipt precedes
// validation, preflight failures carry a safe category, and a
// successful spawn records dispatch-started with outcome unknown — a
// spawned child is not proof a browser opened or a page loaded.
void ForwardURLsShim(RecentActivity *activity, NSString *entryPoint,
	NSArray<NSString *> *urls) {
	for (NSString *url in urls) {
		// Receipt is recorded before any validation, so the menu can
		// answer whether an event even arrived.
		[activity recordKind:@"receipt" entryPoint:entryPoint
			errorCategory:nil detail:-1];

		// Privacy: log the event, never the URL. Query strings and
		// fragments can carry secrets; the browser's own history and
		// the destination server are the record of what was opened.
		AppendLog(@"forwarding URL event");

		NSString *brouter = EmbeddedBrouterPath();
		NSString *config = ConfigPath();
		if (config == nil) {
			AppendLog(@"error: cannot determine the user config directory: "
				@"set $XDG_CONFIG_HOME to an absolute path, or set $HOME");
			[activity recordKind:@"preflight" entryPoint:entryPoint
				errorCategory:@"unavailable-config-directory" detail:-1];
			continue;
		}
		if (![[NSFileManager defaultManager] isExecutableFileAtPath:brouter]) {
			AppendLog([NSString stringWithFormat:@"error: embedded brouter missing at %@", brouter]);
			[activity recordKind:@"preflight" entryPoint:entryPoint
				errorCategory:@"missing-embedded-binary" detail:-1];
			continue;
		}
		[activity recordKind:@"preflight" entryPoint:entryPoint
			errorCategory:nil detail:-1];

		NSString *log = HandlerLogPath();
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
			[activity recordKind:@"dispatch-failed" entryPoint:entryPoint
				errorCategory:@"spawn-failed" detail:spawnErr];
		} else {
			// Fire-and-forget: the child's later outcome (config
			// validation, browser resolution, page load) is not
			// observable here and stays unknown by design.
			[activity recordKind:@"dispatch-started" entryPoint:entryPoint
				errorCategory:nil detail:-1];
		}
	}
}

// PresenceController owns the single menu-bar status item. It lives on
// the same NSApplication loop that delivers URL events; it is created
// once per app instance, so repeated OS opens never add items. The
// menu identifies the app as running — nothing more: health, routing
// success, and default-browser status are deliberately not claimed,
// and no polling runs behind this slice.
@interface PresenceController : NSObject <NSMenuDelegate>
- (instancetype)initWithActivity:(RecentActivity *)activity;
- (void)install;
- (NSMenu *)makeMenu;
- (void)quit:(id)sender;
@end

@implementation PresenceController {
	// Retained for the app lifetime: an unretained NSStatusItem is
	// deallocated and its menu-bar icon vanishes.
	NSStatusItem *_statusItem;
	RecentActivity *_activity;
	NSMenu *_activitySubmenu;
	NSMenuItem *_revealConfigItem;
	NSMenuItem *_revealLogItem;
}

- (instancetype)initWithActivity:(RecentActivity *)activity {
	if (self = [super init]) {
		_activity = activity;
	}
	return self;
}

// Builds the static menu structure. Extracted so the native self-test
// harness can assert the Quit wiring exactly as AppKit dispatches it
// (sendAction:to:from:) without a status bar.
- (NSMenu *)makeMenu {
	NSMenu *menu = [[NSMenu alloc] init];
	NSMenuItem *running = [[NSMenuItem alloc]
		initWithTitle:@"Brouter — running" action:nil keyEquivalent:@""];
	running.enabled = NO;
	[menu addItem:running];
	[menu addItem:[NSMenuItem separatorItem]];

	// Recent Activity is rebuilt when the menu opens (menuNeedsUpdate),
	// so bursts of events never rebuild UI mid-flight and the menu
	// stays responsive.
	NSMenuItem *activityItem = [[NSMenuItem alloc]
		initWithTitle:@"Recent Activity" action:nil keyEquivalent:@""];
	_activitySubmenu = [[NSMenu alloc] init];
	_activitySubmenu.delegate = self;
	activityItem.submenu = _activitySubmenu;
	[menu addItem:activityItem];

	// Reveal items use the already-resolved paths; they never create or
	// overwrite files. menuNeedsUpdate disables them with a (missing)
	// suffix when the target does not exist.
	_revealConfigItem = [[NSMenuItem alloc]
		initWithTitle:@"Reveal Config" action:@selector(reveal:) keyEquivalent:@""];
	_revealConfigItem.target = self;
	_revealConfigItem.representedObject = @"config";
	[menu addItem:_revealConfigItem];
	_revealLogItem = [[NSMenuItem alloc]
		initWithTitle:@"Reveal Diagnostic Log" action:@selector(reveal:) keyEquivalent:@""];
	_revealLogItem.target = self;
	_revealLogItem.representedObject = @"log";
	[menu addItem:_revealLogItem];

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

	// User-selected menu icon (TASK-0026 preview candidate C): a
	// lowercase-b monogram drawn as one continuous stroke ending in an
	// upward arrowhead, shipped as 1x/2x bundle PNGs. Template
	// monochrome: AppKit recolors it for light and dark menu bars.
	NSImage *icon = [NSImage imageNamed:@"menu-icon"];
	if (icon != nil) {
		icon.template = YES;
		icon.size = NSMakeSize(16, 16);
		_statusItem.button.image = icon;
	} else {
		// Resilience: never leave the status item silent and empty.
		AppendLog(@"error: menu icon asset missing; falling back to system symbol");
		icon = [NSImage imageWithSystemSymbolName:@"arrow.triangle.branch"
			accessibilityDescription:@"Brouter"];
		if (icon != nil) {
			icon.template = YES;
			_statusItem.button.image = icon;
		}
	}
	_statusItem.button.accessibilityLabel = @"Brouter";
	_statusItem.button.toolTip = @"Brouter — running";
	_statusItem.menu = [self makeMenu];

	AppendLog(@"menu bar presence installed");
}

// Rebuilds the Recent Activity submenu at open time from the bounded
// store: newest first, empty state when nothing happened this session.
- (void)menuNeedsUpdate:(NSMenu *)menu {
	if (menu == _activitySubmenu) {
		[menu removeAllItems];
		NSArray<ActivityRecord *> *entries = [_activity entries];
		if (entries.count == 0) {
			NSMenuItem *empty = [[NSMenuItem alloc]
				initWithTitle:@"No activity this session" action:nil keyEquivalent:@""];
			empty.enabled = NO;
			[menu addItem:empty];
		} else {
			for (ActivityRecord *entry in entries) {
				NSMenuItem *item = [[NSMenuItem alloc]
					initWithTitle:[_activity displayLineForEntry:entry]
					action:nil keyEquivalent:@""];
				item.enabled = NO; // informative rows; not buttons
				[menu addItem:item];
			}
		}
		[menu addItem:[NSMenuItem separatorItem]];
		// Memory only: never touches diagnostic files, and cannot cancel
		// already-dispatched (fire-and-forget) children.
		NSMenuItem *clear = [[NSMenuItem alloc]
			initWithTitle:@"Clear Recent Activity" action:@selector(clearActivity:)
			keyEquivalent:@""];
		clear.target = self;
		[menu addItem:clear];
		return;
	}

	// Reveal items: enabled only when the target already exists; the
	// (missing) suffix is actionable, non-secret feedback. Files are
	// never created or overwritten by the menu.
	NSString *configPath = ConfigPath();
	BOOL configExists = configPath != nil &&
		[[NSFileManager defaultManager] fileExistsAtPath:configPath];
	_revealConfigItem.enabled = configExists;
	_revealConfigItem.title = configExists ? @"Reveal Config" : @"Reveal Config (missing)";

	NSString *logPath = HandlerLogPath();
	BOOL logExists = [[NSFileManager defaultManager] fileExistsAtPath:logPath];
	_revealLogItem.enabled = logExists;
	_revealLogItem.title = logExists ? @"Reveal Diagnostic Log" : @"Reveal Diagnostic Log (missing)";
}

- (void)reveal:(NSMenuItem *)sender {
	NSString *path = nil;
	if ([sender.representedObject isEqualToString:@"config"]) {
		path = ConfigPath();
	} else if ([sender.representedObject isEqualToString:@"log"]) {
		path = HandlerLogPath();
	}
	if (path == nil || ![[NSFileManager defaultManager] fileExistsAtPath:path]) {
		// Defensive: the menu already disables missing targets. Return
		// without writing — AppendLog would CREATE the very file the
		// reveal contract says must never be created.
		return;
	}
	// activateFileViewerSelectingURLs takes nil-safe arguments and
	// reveals without opening or modifying the file.
	[[NSWorkspace sharedWorkspace]
		activateFileViewerSelectingURLs:@[[NSURL fileURLWithPath:path]]];
}

- (void)clearActivity:(id)sender {
	[_activity clear];
}

- (void)quit:(id)sender {
	[[NSApplication sharedApplication] terminate:self];
}

@end

@interface EventRedirector : NSObject
- (instancetype)initWithActivity:(RecentActivity *)activity;
- (void)handleGetURLEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply;
@end

@implementation EventRedirector {
	RecentActivity *_activity;
}

- (instancetype)initWithActivity:(RecentActivity *)activity {
	if (self = [super init]) {
		_activity = activity;
	}
	return self;
}

- (void)handleGetURLEvent:(NSAppleEventDescriptor *)event withReplyEvent:(NSAppleEventDescriptor *)reply {
	NSString *url = [[event descriptorForKeyword:keyDirectObject] stringValue];
	if (url.length > 0) {
		ForwardURLsShim(_activity, @"os-event", @[url]);
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
			RecentActivity *activity = [[RecentActivity alloc] init];
			ForwardURLsShim(activity, @"argv", argvURLs);
			return 0;
		}

		// Launched as a long-running handler. LaunchServices delivers the
		// GURL Apple Event only to an initialized NSApplication: a bare
		// NSRunLoop never services the event queue, and `open -a` fails
		// with the -1712 timeout (observed defect). NSApplication's run
		// loop registers with the Window Server and dispatches Apple
		// Events to the NSAppleEventManager handler.
		NSApplication *application = [NSApplication sharedApplication];
		RecentActivity *activity = [[RecentActivity alloc] init];
		EventRedirector *redirector = [[EventRedirector alloc] initWithActivity:activity];
		[[NSAppleEventManager sharedAppleEventManager]
			setEventHandler:redirector
				andSelector:@selector(handleGetURLEvent:withReplyEvent:)
				forEventClass:kInternetEventClass
				   andEventID:kAEGetURL];

		// Menu-bar presence rides the same run loop and application:
		// no daemon, no tray framework, no focus change, no Dock icon.
		PresenceController *presence = [[PresenceController alloc] initWithActivity:activity];
		[presence install];

		AppendLog(@"handler started; waiting for URL events");
		[application run];
		return 0;
	}
}
