// openfiles_selftest.m is the focused native seam for TASK-0033 local
// documents. It is compiled ad hoc by scripts/macos_openfiles_test.go
// together with the shim source (the shim entry point is renamed on
// the compiler command line).
//
// Mode: warm — drives OpenDocumentsDelegate's application:open: the
// same way NSApplication delivers kAEOpenDocuments on a running
// handler, with several file URLs (spaces and unicode names). A stub
// "brouter" beside the harness binary records the forwarded arguments,
// so the test proves the warm path forwards one spawn per document
// with the encoded file URLs — without LaunchServices, a real install,
// or a GUI. Exit 0 on success, 3 on any failed assertion.
#import <AppKit/AppKit.h>
#import <Foundation/Foundation.h>

@class RecentActivity;

// Provided by the shim source linked into this harness.
void ForwardURLsShim(RecentActivity *activity, NSString *entryPoint,
	NSArray<NSString *> *urls);

@interface OpenDocumentsDelegate : NSObject <NSApplicationDelegate>
- (instancetype)initWithActivity:(RecentActivity *)activity;
- (void)application:(NSApplication *)application open:(NSArray<NSURL *> *)urls;
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
		if (argc < 2) {
			return Fail("documents directory argument required");
		}
		NSString *logPath = [NSString stringWithUTF8String:getenv("STUB_ARGV_LOG")];

		NSString *docDir = [NSString stringWithUTF8String:argv[1]];
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

		// Warm-path delivery, exactly what NSApplication does when the
		// running handler is handed documents: application:open: with
		// file URLs, potentially several at once.
		[delegate application:NSApplication.sharedApplication open:@[first, second]];

		// The shim spawns asynchronously; poll for the stub's log.
		NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:5.0];
		NSString *logged = nil;
		while ([[NSDate date] compare:deadline] == NSOrderedAscending) {
			logged = ReadFile(logPath);
			if (logged && [logged componentsSeparatedByString:@"\n"].count >= 3) {
				break;
			}
			[NSThread sleepForTimeInterval:0.05];
		}
		if (!logged) {
			return Fail("stub argv log never appeared");
		}

		// Each spawn appends its whole argv (open, --config, cfg, URL);
		// assert the two documents arrived, encoded, one spawn each.
		NSArray *lines = [logged componentsSeparatedByString:@"\n"];
		NSString *wantFirst = [first absoluteString];
		NSString *wantSecond = [second absoluteString];
		int firstCount = 0, secondCount = 0;
		for (NSString *line in lines) {
			if ([line isEqualToString:wantFirst]) firstCount++;
			if ([line isEqualToString:wantSecond]) secondCount++;
		}
		if (firstCount != 1 || secondCount != 1) {
			fprintf(stderr, "FAIL forwards first=%d second=%d\n", firstCount, secondCount);
			return 3;
		}

		// Privacy contract on the warm path: the handler log records the
		// event, never the document path.
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
