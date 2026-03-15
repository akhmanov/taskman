package model

func IsSupportedTransitionVerb(verb string) bool {
	switch verb {
	case "plan", "start", "pause", "resume", "complete", "cancel", "reopen":
		return true
	default:
		return false
	}
}
