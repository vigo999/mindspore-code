package loop

import "testing"

func TestPermissionManager_ApproveOnceFlow(t *testing.T) {
	pm := NewPermissionManager(false, nil)

	ok, err := pm.Request("shell", "rm -rf /tmp/mscli-test", "")
	if ok {
		t.Fatal("expected dangerous shell to require approval")
	}
	if err == nil {
		t.Fatal("expected approval required error")
	}

	if _, err := pm.ApproveOncePending(); err != nil {
		t.Fatalf("ApproveOncePending failed: %v", err)
	}

	ok, err = pm.Request("shell", "rm -rf /tmp/mscli-test", "")
	if err != nil || !ok {
		t.Fatalf("expected once-approved request to pass, ok=%v err=%v", ok, err)
	}

	ok, err = pm.Request("shell", "rm -rf /tmp/mscli-test", "")
	if ok || err == nil {
		t.Fatal("expected approval requirement after once approval consumed")
	}
}

func TestPermissionManager_WhitelistBlacklistAndYolo(t *testing.T) {
	pm := NewPermissionManager(false, []string{"read", "grep"})

	ok, err := pm.Request("read", "a.txt", "a.txt")
	if err != nil || !ok {
		t.Fatalf("expected read to be allowed, ok=%v err=%v", ok, err)
	}

	ok, err = pm.Request("edit", "a.txt", "a.txt")
	if ok || err == nil {
		t.Fatalf("expected edit denied by whitelist, ok=%v err=%v", ok, err)
	}

	pm.AddBlacklist("read")
	ok, err = pm.Request("read", "a.txt", "a.txt")
	if ok || err == nil {
		t.Fatalf("expected read denied by blacklist, ok=%v err=%v", ok, err)
	}

	pm.SetYolo(true)
	ok, err = pm.Request("shell", "rm -rf /tmp/x", "")
	if err != nil || !ok {
		t.Fatalf("expected yolo to allow, ok=%v err=%v", ok, err)
	}
}

func TestPermissionManager_SafeShellNoApproval(t *testing.T) {
	pm := NewPermissionManager(false, nil)
	ok, err := pm.Request("shell", "ls -la", "")
	if err != nil || !ok {
		t.Fatalf("safe shell command should be allowed without approval, ok=%v err=%v", ok, err)
	}
}
