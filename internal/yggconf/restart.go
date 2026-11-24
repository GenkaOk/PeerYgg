package yggconf

// RestartCommand returns the command that restarts yggdrasil for the given
// GOOS. PeerYgg never restarts the service itself; the CLI prints this command
// instead. known is false when the platform has no prepared command and the
// returned string is only a generic hint.
func RestartCommand(goos string) (cmd string, known bool) {
	switch goos {
	case "linux":
		return "systemctl restart yggdrasil", true
	case "darwin":
		return "launchctl kickstart -k system/yggdrasil", true
	case "freebsd":
		return "service yggdrasil restart", true
	case "windows":
		return "Restart-Service Yggdrasil", true
	default:
		return "restart the yggdrasil service with your init system (for example: rc-service yggdrasil restart)", false
	}
}
