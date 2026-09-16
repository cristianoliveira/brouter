// openfiles_selftest.m is the focused native seam for TASK-0033 local
// documents. It is compiled ad hoc by scripts/macos_openfiles_test.go
// together with the shim source (the shim entry point is renamed on
// the compiler command line).
//
// What this harness proves: the delegate wiring, lifetime, and the
// EXACT AppKit delivery selector from the local SDK header —
// -application:openURLs: (NSURL array, macOS 10.13+), with the
// deprecated -application:openFiles:/-application:openFile: absent so
// AppKit has one dispatch target and a document can never be delivered
// twice. Driving the selector with file URLs (spaces in names) must
// forward one batched spawn through a stub embedded brouter with the
// redacted handler log — no LaunchServices registration, no real
// install, no GUI. The hop before this call (real LaunchServices
// handoff to an installed, running bundle) cannot be reached without
// live state and is recorded UNTESTED.
#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>

@class RecentActivity;

// Provided by the shim source linked into this harness.
void ForwardURLsShim(RecentActivity *activity, NSString *entryPoint,
	NSArray<NSString *> *urls);
void ForwardDocumentsShim(RecentActivity *activity, NSString *entryPoint,
	NSArray<NSString *> *paths);

@interface OpenDocumentsDelegate : NSObject <NSApplicationDelegate>
- (instancetype)initWithActivity:(RecentActivity *)activity;
- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls;
@end

@interface RecentActivity : NSObject
- (instancetype)initWithCapacity:(NSUInteger)capacity;
@end

static int Fail(const char *message) {
	fprintf(stderr, "FAIL %s\n", message);
	return 3;
}

static NSString *ReadFile(NSString *path) {
	NSError *error = nil;
	NSString *content = [NSString stringWithContentsOfFile:path
		encoding:NSUTF8StringEncoding error:&error];
	return error ? nil : content;
}

int main(int argc, const char *argv[]) {
	@autoreleasepool {
		if (argc < 3) {
			return Fail("mode and documents directory arguments required");
		}
		NSString *mode = [NSString stringWithUTF8String:argv[1]];
		NSString *logPath = [NSString stringWithUTF8String:getenv("STUB_ARGV_LOG")];

		NSString *docDir = [NSString stringWithUTF8String:argv[2]];
		// Build the documents from the directory's own names: the
		// filesystem's unicode spelling is the contract, not this
		// source literal's.
		NSError *enumError = nil;
		NSArray *names = [[NSFileManager defaultManager]
			contentsOfDirectoryAtPath:docDir error:&enumError];
		if (enumError || names.count != 2) {
			return Fail("fixture documents not found");
		}
		NSURL *first = [NSURL fileURLWithPath:[docDir stringByAppendingPathComponent:names[0]]];
		NSURL *second = [NSURL fileURLWithPath:[docDir stringByAppendingPathComponent:names[1]]];

		RecentActivity *activity = [[RecentActivity alloc] initWithCapacity:20];
		OpenDocumentsDelegate *delegate = [[OpenDocumentsDelegate alloc] initWithActivity:activity];

		// Lifetime: NSApplication holds its delegate weakly, so main()
		// owns the delegate through its stack reference. Mirror that
		// wiring here and assert the installed identity.
		NSApplication *application = [NSApplication sharedApplication];
		application.delegate = delegate;
		if (application.delegate != (id<NSApplicationDelegate>)delegate) {
			return Fail("delegate not installed on NSApplication");
		}
		// NSApplication queues document events until finishLaunching:
		// without it the odoc handler is not installed. main() reaches
		// finishLaunching inside [application run]; the harness does it
		// explicitly.
		[application finishLaunching];

		// Exact delivery surface per the local AppKit SDK header:
		// -application:openURLs: implemented;
		// deprecated -application:openFiles: and singular
		// -application:openFile: absent (the header says implementing
		// openURLs: suppresses both — one dispatch target, no double
		// delivery).
		if (![delegate respondsToSelector:@selector(application:openURLs:)]) {
			return Fail("application:openURLs: not implemented");
		}
		if ([delegate respondsToSelector:@selector(application:openFiles:)]) {
			return Fail("deprecated application:openFiles: must not be implemented");
		}
		if ([delegate respondsToSelector:@selector(application:openFile:)]) {
			return Fail("deprecated application:openFile: must not be implemented");
		}

		if ([mode isEqualToString:@"warm-dispatch"]) {
			// REAL dispatch: build a genuine kAEOpenDocuments Apple
			// Event (direct object = a typeFileURL list) and hand it to
			// NSAppleEventManager's public raw dispatch — the same
			// machinery NSApplication services for the OS. The event
			// routes through AppKit's installed odoc handler to the
			// delegate selector below; nothing here is a direct
			// selector call, and no LaunchServices state is touched.
			ProcessSerialNumber psn = {0, kCurrentProcess};
			NSAppleEventDescriptor *target =
				[NSAppleEventDescriptor descriptorWithDescriptorType:typeProcessSerialNumber
					bytes:&psn length:sizeof(psn)];
			NSAppleEventDescriptor *event =
				[NSAppleEventDescriptor appleEventWithEventClass:kCoreEventClass
					eventID:kAEOpenDocuments
					targetDescriptor:target
					returnID:kAutoGenerateReturnID
					transactionID:kAnyTransactionID];
			NSAppleEventDescriptor *list = [NSAppleEventDescriptor listDescriptor];
			NSUInteger index = 1;
			for (NSURL *url in @[first, second]) {
				NSData *data = [[url absoluteString] dataUsingEncoding:NSUTF8StringEncoding];
				[list insertDescriptor:[NSAppleEventDescriptor
					descriptorWithDescriptorType:typeFileURL data:data]
					atIndex:index++];
			}
			[event setParamDescriptor:list forKeyword:keyDirectObject];

			AppleEvent reply;
			if (AECreateDesc(typeNull, NULL, 0, &reply) != noErr) {
				return Fail("reply descriptor init failed");
			}
			OSStatus status = [[NSAppleEventManager sharedAppleEventManager]
				dispatchRawAppleEvent:[event aeDesc]
				withRawReply:&reply
				handlerRefCon:NULL];
			AEDisposeDesc(&reply);
			if (status != noErr) {
				fprintf(stderr, "FAIL dispatchRawAppleEvent (%d)\n", status);
				return 3;
			}
		} else if ([mode isEqualToString:@"warm-mixed"]) {
			// The header hands openURLs: ANY URLs — documents AND web
			// schemes. A mixed array must split: documents batch into
			// one spawn; the web URL keeps its per-URL spawn.
			NSURL *web = [NSURL URLWithString:@"https://mixed.example/x"];
			[delegate application:application openURLs:@[first, web, second]];
		} else {
			// Delegate-contract delivery: the exact selector AppKit
			// itself calls after servicing a kAEOpenDocuments event,
			// with several file URLs at once.
			[delegate application:application openURLs:@[first, second]];
		}

		// The shim spawns asynchronously; poll for the stub's log.
		NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:5.0];
		NSString *logged = nil;
		while ([NSDate date].timeIntervalSinceReferenceDate <
			deadline.timeIntervalSinceReferenceDate) {
			logged = ReadFile(logPath);
			if (logged && [logged componentsSeparatedByString:@"\n"].count >= 3) {
				break;
			}
			[NSThread sleepForTimeInterval:0.05];
		}
		if (!logged) {
			return Fail("stub argv log never appeared");
		}

		// Each spawn appends its whole argv (open, --config, cfg, URLs);
		// assert the two documents arrived in ONE batched spawn.
		if (ReadFile(logPath) == nil) {
			return Fail("stub argv log unreadable");
		}
		NSString *wantFirst = [first absoluteString];
		NSString *wantSecond = [second absoluteString];
		NSString *wantWeb = @"https://mixed.example/x";
		int firstCount = 0, secondCount = 0, spawnCount = 0, webCount = 0;
		for (NSString *line in [logged componentsSeparatedByString:@"\n"]) {
			if ([line isEqualToString:@"open"]) spawnCount++;
			if ([line isEqualToString:wantFirst]) firstCount++;
			if ([line isEqualToString:wantSecond]) secondCount++;
			if ([line isEqualToString:wantWeb]) webCount++;
		}
		// Documents must arrive as ONE batched spawn. In the mixed
		// scenario the web URL adds exactly one more spawn of its own.
		int wantSpawns = [mode isEqualToString:@"warm-mixed"] ? 2 : 1;
		if (spawnCount != wantSpawns) {
			fprintf(stderr, "FAIL spawns=%d want=%d\n", spawnCount, wantSpawns);
			return 3;
		}
		if (firstCount != 1 || secondCount != 1) {
			fprintf(stderr, "FAIL forwards first=%d second=%d\n", firstCount, secondCount);
			return 3;
		}
		if ([mode isEqualToString:@"warm-mixed"] && webCount != 1) {
			fprintf(stderr, "FAIL web forwards=%d want=1\n", webCount);
			return 3;
		}

		// Privacy contract on the warm path: the event is logged, the
		// document path is not.
		NSString *handlerLog = ReadFile([NSString stringWithUTF8String:getenv("BRROUTER_HANDLER_LOG")]);
		if (!handlerLog || ![handlerLog containsString:@"forwarding URL event"]) {
			return Fail("handler log missing the redacted event marker");
		}
		if ([handlerLog containsString:docDir]) {
			return Fail("handler log leaks the document path");
		}
		return 0;
	}
}
