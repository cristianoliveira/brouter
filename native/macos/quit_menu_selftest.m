// macos_quit_menu_selftest.m is a focused native seam for the Quit
// menu wiring (TASK-0022). It is compiled ad hoc by
// TestMacosQuitMenuActionDispatchesTermination together with the shim
// source (its main is renamed on the compiler command line) and
// asserts the wiring the way AppKit itself dispatches a menu click:
// NSApp sendAction:to:from:. A target that does not respond to its
// item's action — the defect QA found: action terminate: on
// PresenceController, which implements only quit: — fails the harness
// with exit code 3 before any dispatch is attempted. A correct wiring
// dispatches quit:, which terminates the harness process (exit 0).
#import <AppKit/AppKit.h>

@interface PresenceController : NSObject
- (NSMenu *)makeMenu;
@end

int main(void) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		PresenceController *presence = [[PresenceController alloc] init];
		NSMenu *menu = [presence makeMenu];

		NSMenuItem *quit = nil;
		NSMenuItem *running = nil;
		for (NSMenuItem *item in menu.itemArray) {
			if ([item.title isEqualToString:@"Quit Brouter"]) {
				quit = item;
			}
			if ([item.title isEqualToString:@"Brouter — running"]) {
				running = item;
			}
		}
		if (quit == nil) {
			fprintf(stderr, "FAIL-NO-QUIT-ITEM\n");
			return 3;
		}
		if (running == nil || running.isEnabled) {
			fprintf(stderr, "FAIL-RUNNING-ITEM (exists=%d enabled=%d)\n",
				running != nil, running.isEnabled);
			return 3;
		}
		BOOL responds = quit.target != nil &&
			[quit.target respondsToSelector:quit.action];
		if (!responds) {
			fprintf(stderr, "FAIL-WIRING responds=%d\n", responds);
			return 3;
		}

		fprintf(stderr, "WIRING-OK dispatching %s\n",
			NSStringFromSelector(quit.action).UTF8String);
		[NSApp sendAction:quit.action to:quit.target from:quit];
		// terminate: must end the process; reaching this line means the
		// action returned without terminating.
		fprintf(stderr, "FAIL-ACTION-RETURNED-WITHOUT-TERMINATING\n");
		return 4;
	}
}
