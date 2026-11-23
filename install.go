//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

	"github.com/dvygolov/yalauncher/internal/win"
)

// Autostart has two flavours:
//
//   - the Run key starts the exe with "--start-task"; that mode only asks Task
//     Scheduler to run the real task. The task carries "highest privileges",
//     so the tray app comes up elevated without a UAC prompt and stays visible
//     in the Windows "Startup apps" list, where it can be switched off.
//   - plain: the Run entry starts the exe directly (used when elevation is
//     declined).
//
// Without elevation the keyboard hook cannot see keys typed into elevated
// windows, which is the whole point of running as admin.

// syncAutostart makes the registry and the scheduled task match the config.
func (a *App) syncAutostart() {
	exe := win.ExePath()
	elevated := win.IsElevated()
	if !a.cfg.Autostart {
		win.DeleteRunValue(appName)
		if elevated {
			deleteLogonTask()
		}
		logf("autostart: disabled")
		return
	}
	if exe == "" {
		logf("autostart: cannot determine the exe path")
		return
	}
	if !win.SetRunValue(appName, exe, "--start-task") {
		logf("autostart: writing the HKCU Run entry failed")
	}
	if !elevated {
		logf("autostart: Run entry set, task will be created on next elevated start")
		return
	}
	if taskExists() && strings.EqualFold(a.cfg.InstalledExe, exe) {
		return
	}
	if err := createLogonTask(exe); err != nil {
		logf("autostart: creating the scheduled task failed: %v", err)
		a.balloon("Автозапуск", "Не удалось создать задачу планировщика, будет запрос UAC при входе.", win.NIIF_WARNING)
		return
	}
	logf("autostart: scheduled task created for %s", exe)
	a.cfg.InstalledExe = exe
	if err := a.cfg.save(); err != nil {
		logf("config: save failed: %v", err)
	}
}

// cleanupOldBinary removes the leftover of a previous self-update.
func cleanupOldBinary() {
	cur := win.ExePath()
	if cur == "" {
		return
	}
	if _, err := os.Stat(cur + ".old"); err == nil {
		if err := os.Remove(cur + ".old"); err == nil {
			logf("update: previous binary removed")
		}
	}
}

// addFolderDialog asks for one folder with the shell's picker. The dialog is
// COM based; SHBrowseForFolder with the modern flag is the smallest way to get
// a native window.
func (a *App) addFolderDialog() {
	dir := browseFolder("Добавить папку в индекс")
	if dir == "" {
		return
	}
	for _, f := range a.cfg.Folders {
		if strings.EqualFold(filepath.Clean(f), filepath.Clean(dir)) {
			a.balloon("Папка уже в индексе", dir, win.NIIF_INFO)
			return
		}
	}
	a.cfg.Folders = append(a.cfg.Folders, dir)
	a.saveConfig()
	a.balloon("Папка добавлена", dir+"\nИндекс перестраивается…", win.NIIF_INFO)
	go a.reindex(false)
}

const createNoWindow = 0x08000000

func runHidden(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func schtasksPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", "schtasks.exe")
}

func taskExists() bool {
	_, err := runHidden(schtasksPath(), "/Query", "/TN", taskName)
	return err == nil
}

// startLogonTask asks Task Scheduler to start the elevated instance. It is the
// only thing "--start-task" does; the traceable autostart entry is the Run key.
func startLogonTask() error {
	out, err := runHidden(schtasksPath(), "/Run", "/TN", taskName)
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(out)
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("schtasks /Run: %s", msg)
}

// createLogonTask (re)registers the elevated launcher. The task has no trigger
// of its own: it is started on demand by the Run entry, so disabling that entry
// really disables the autostart.
func createLogonTask(exe string) error {
	path := filepath.Join(os.TempDir(), "yalauncher-task.xml")
	if err := os.WriteFile(path, taskXML(exe), 0o644); err != nil {
		return err
	}
	defer os.Remove(path)
	out, err := runHidden(schtasksPath(), "/Create", "/F", "/TN", taskName, "/XML", path)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("schtasks /Create: %s", msg)
	}
	return nil
}

func deleteLogonTask() {
	if _, err := runHidden(schtasksPath(), "/Delete", "/F", "/TN", taskName); err != nil {
		logf("autostart: deleting the scheduled task failed: %v", err)
	}
}

// taskXML mirrors what Task Scheduler itself exports for such a task: highest
// privileges, no trigger, no execution time limit, one instance at a time.
func taskXML(exe string) []byte {
	user := os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	body := `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Author>YALauncher</Author>
    <Description>Starts YALauncher with administrator rights without a UAC prompt.</Description>
    <URI>\` + taskName + `</URI>
  </RegistrationInfo>
  <Principals>
    <Principal id="Author">
      <UserId>` + xmlEscape(user) + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
  </Settings>
  <Triggers />
  <Actions Context="Author">
    <Exec>
      <Command>` + xmlEscape(exe) + `</Command>
    </Exec>
  </Actions>
</Task>
`
	units := utf16.Encode([]rune(body))
	out := make([]byte, 0, 2+len(units)*2)
	out = append(out, 0xFF, 0xFE)
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
