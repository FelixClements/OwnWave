package pipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotifyTrigger(t *testing.T) {
	require.Equal(t, "genres_scored", NotifyTrigger("genres_scored"))
	require.Equal(t, "scan_finished", NotifyTrigger("scan_finished"))
	for _, bad := range []string{"", "Genres", "drop table;", "x-y", "a_very_long_trigger_name_that_goes_on_and_on"} {
		require.Equal(t, "notify", NotifyTrigger(bad), bad)
	}
}

func TestTriggerFoldsIntoAWaitingRun(t *testing.T) {
	r := &Runner{kick: make(chan string, 1)}
	r.Trigger("scan_finished")
	r.Trigger("genres_scored") // folds into the waiting run
	require.Equal(t, "scan_finished", <-r.kick)
	select {
	case got := <-r.kick:
		t.Fatalf("unexpected second run %q", got)
	default:
	}
}
