package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func LockWorkflowWorkspace(stateDirectory string) (func(), error) {
	if err := os.MkdirAll(stateDirectory, 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(stateDirectory, "workflow.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("another workflow activity owns this project's workspace; inspect it before retrying")
	}
	if data, readErr := os.ReadFile(filepath.Join(stateDirectory, "workflow-active-process.json")); readErr == nil {
		var processJournal ActiveProcess
		if json.Unmarshal(data, &processJournal) != nil || processJournal.PID <= 0 {
			file.Close()
			return nil, errors.New("invalid active-process journal; inspect it before retrying")
		}
		err = syscall.Kill(-processJournal.PID, 0)
		if err == nil || !errors.Is(err, syscall.ESRCH) {
			file.Close()
			return nil, fmt.Errorf("an earlier activity process group %d may still be running; stop or inspect it before retrying", processJournal.PID)
		}
		if err = os.Remove(filepath.Join(stateDirectory, "workflow-active-process.json")); err != nil {
			file.Close()
			return nil, err
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		file.Close()
		return nil, readErr
	}
	return func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN); _ = file.Close() }, nil
}
