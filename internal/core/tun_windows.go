package core

import (
	"fmt"
	"os/exec"

	"tungo/internal/spawn"
)

func resetTun() {
	script := fmt.Sprintf(`$ErrorActionPreference='SilentlyContinue'
function Left {
  Get-CimInstance Win32_PnPEntity | Where-Object {
    $_.DeviceID -like 'SWD\WINTUN*' -and $_.Status -ne 'OK' -and $_.Name -like '*%s*'
  }
}
$t = [Diagnostics.Stopwatch]::StartNew()
while ($t.Elapsed.TotalSeconds -lt 10) {
  $left = @(Left)
  if ($left.Count -eq 0) { break }
  foreach ($one in $left) { & pnputil /remove-device $one.DeviceID | Out-Null }
  Start-Sleep -Milliseconds 400
}
Start-Sleep -Milliseconds 300
`, tunInterface)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	spawn.Hide(cmd)
	cmd.Run()
}
