// activity_selftest.m is the focused native seam for TASK-0023 Recent
// Activity. It is compiled ad hoc by scripts/macos_activity_test.go
// together with the shim source (the shim entry point is renamed on
// the compiler command line).
//
// Modes:
//   unit    — in-process assertions for empty state, bounded retention
//             with deterministic oldest-first eviction, newest-first
//             order, injected clock and request IDs, and Clear. Exit 0
//             on success, 3 on any failed assertion.
//   forward — configures a real ForwardURLs run (receipt, preflight,
//             dispatch) for one scenario chosen by the parent, prints
//             the resulting activity lines (allowlisted fields only)
//             to stdout, and exits 0. The parent asserts which
//             categories appear and that untrusted markers never do.
#import <AppKit/AppKit.h>

@class ActivityRecord;
@interface RecentActivity : NSObject
- (instancetype)initWithCapacity:(NSUInteger)capacity;
- (void)recordKind:(NSString *)kind
	entryPoint:(NSString *)entryPoint
	errorCategory:(NSString *)errorCategory
	detail:(NSInteger)detail;
- (NSArray *)entries; // newest first
- (void)clear;
- (NSString *)displayLineForEntry:(id)entry;
@property (copy) NSDate *(^now)(void);
@end

// Provided by the shim source linked into this harness.
void ForwardURLsShim(RecentActivity *activity, NSString *entryPoint,
	NSArray<NSString *> *urls);

static int Fail(const char *message) {
	fprintf(stderr, "FAIL %s\n", message);
	return 3;
}

int main(int argc, const char *argv[]) {
	@autoreleasepool {
		if (argc < 2) {
			return Fail("mode argument required");
		}
		NSString *mode = [NSString stringWithUTF8String:argv[1]];

		if ([mode isEqualToString:@"unit"]) {
			RecentActivity *activity = [[RecentActivity alloc] initWithCapacity:20];

			// Empty state before any event.
			if ([activity.entries count] != 0) {
				return Fail("new activity must be empty");
			}

			// Injected clock and request IDs make rendering deterministic.
			NSDateFormatter *formatter = [[NSDateFormatter alloc] init];
			formatter.dateFormat = @"HH:mm:ss";
			formatter.timeZone = [NSTimeZone timeZoneForSecondsFromGMT:0];
			__block double seconds = 0;
			activity.now = ^NSDate * {
				return [NSDate dateWithTimeIntervalSince1970:seconds];
			};

			// Burst past capacity: 35 receipts into a 20-slot store.
			for (int i = 1; i <= 35; i++) {
				seconds = i;
				[activity recordKind:@"receipt" entryPoint:@"os-event"
					errorCategory:nil detail:-1];
			}
			if ([activity.entries count] != 20) {
				fprintf(stderr, "FAIL capacity %lu != 20\n", (unsigned long)[activity.entries count]);
				return 3;
			}
			// Newest first, oldest evicted deterministically.
			NSString *newest = [activity displayLineForEntry:[activity.entries firstObject]];
			NSString *oldest = [activity displayLineForEntry:[activity.entries lastObject]];
			if (![newest containsString:@"#35"] || ![oldest containsString:@"#16"]) {
				fprintf(stderr, "FAIL eviction newest=%s oldest=%s\n",
					newest.UTF8String, oldest.UTF8String);
				return 3;
			}

			// Clear removes entries only; recording continues afterwards.
			[activity clear];
			if ([activity.entries count] != 0) {
				return Fail("clear must empty the entries");
			}
			seconds = 100;
			[activity recordKind:@"receipt" entryPoint:@"argv"
				errorCategory:nil detail:-1];
			if ([activity.entries count] != 1 ||
				![[activity displayLineForEntry:[activity.entries firstObject]]
					containsString:@"#36"]) {
				return Fail("recording must continue after clear");
			}
			return 0;
		}

		if ([mode isEqualToString:@"forward"]) {
			if (argc < 4) {
				return Fail("forward mode: entryPoint and URL required");
			}
			NSString *entryPoint = [NSString stringWithUTF8String:argv[2]];
			NSString *url = [NSString stringWithUTF8String:argv[3]];
			RecentActivity *activity = [[RecentActivity alloc] init];
			// The harness executable stands in for the bundle shim: the
			// embedded-brouter lookup resolves beside argv[0], so the
			// parent controls the preflight outcome per scenario.
			ForwardURLsShim(activity, entryPoint, @[url]);
			for (id entry in [activity entries]) {
				printf("%s\n", [activity displayLineForEntry:entry].UTF8String);
			}
			return 0;
		}

		return Fail("unknown mode");
	}
}
