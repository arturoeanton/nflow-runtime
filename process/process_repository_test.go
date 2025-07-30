package process

import (
	"sync"
	"testing"
	"time"
)

func TestProcessRepositoryConcurrency(t *testing.T) {
	repo := NewProcessRepository()

	// Test concurrent writes
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				wid := string(rune('A'+id%26)) + string(rune('0'+j%10))
				p := &Process{
					UUID:           wid,
					State:          "running",
					UUIDBoxCurrent: "box-" + wid,
					Type:           "test",
					Payload:        map[string]interface{}{"id": id, "iter": j},
					Killeable:      true,
				}
				repo.Set(wid, p)

				// Concurrent read
				if retrieved, exists := repo.Get(wid); exists {
					if retrieved.UUID != wid {
						t.Errorf("Retrieved wrong process: expected %s, got %s", wid, retrieved.UUID)
					}
				}

				// Random delete
				if j%5 == 0 {
					repo.Delete(wid)
				}
			}
		}(i)
	}

	wg.Wait()

	// Test GetAll doesn't cause race conditions
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				all := repo.GetAll()
				// Just access the map to ensure no race conditions
				_ = len(all)
			}
		}()
	}

	wg.Wait()

	// Test GetAllKeys concurrent access
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				keys := repo.GetAllKeys()
				_ = len(keys)
			}
		}()
	}

	wg.Wait()
}

func TestProcessRepositoryIntegration(t *testing.T) {
	// Inicializar repository global
	InitializeRepository()

	// Test with global repository
	wid := "test-123"
	p := CreateProcess(wid)

	if p.UUID != wid {
		t.Errorf("Process UUID mismatch: expected %s, got %s", wid, p.UUID)
	}

	// Test Get
	retrieved, exists := GetProcessID(wid)
	if !exists {
		t.Error("Process should exist")
	}
	if retrieved.UUID != wid {
		t.Errorf("Retrieved wrong process: expected %s, got %s", wid, retrieved.UUID)
	}

	// Test concurrent access to global repository
	var wg sync.WaitGroup
	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func(id int) {
			defer wg.Done()
			wid := string(rune('A' + id%26))
			p := CreateProcessWithCallback(wid)
			time.Sleep(time.Millisecond)
			p.Kill()
		}(i)
	}

	wg.Wait()

	// Clean up
	repo := GetRepository()
	repo.Clear()
}

func TestProcessSendCallback(t *testing.T) {
	p := &Process{
		UUID:     "test-callback",
		Callback: make(chan string, 1),
	}

	// Test SendCallback
	testData := `{"status":"complete"}`
	go func() {
		p.SendCallback(testData)
	}()

	select {
	case received := <-p.Callback:
		if received != testData {
			t.Errorf("Callback data mismatch: expected %s, got %s", testData, received)
		}
	case <-time.After(time.Second):
		t.Error("Callback timeout")
	}
}

func TestWKillConcurrency(t *testing.T) {
	InitializeRepository()
	repo := GetRepository()
	repo.Clear()

	// Create multiple processes
	var wg sync.WaitGroup
	numProcesses := 20

	for i := 0; i < numProcesses; i++ {
		wid := string(rune('A' + i))
		CreateProcessWithCallback(wid)
		// No establecer websocket mock - WKill manejará nil correctamente
	}

	// Kill all processes concurrently
	wg.Add(numProcesses * 2)
	for i := 0; i < numProcesses; i++ {
		go func(id int) {
			defer wg.Done()
			wid := string(rune('A' + id))
			WKill(wid)
		}(i)

		// Also try to access while killing
		go func(id int) {
			defer wg.Done()
			wid := string(rune('A' + id))
			GetProcessID(wid)
		}(i)
	}

	wg.Wait()

	// Verify all processes are marked as killed
	for i := 0; i < numProcesses; i++ {
		wid := string(rune('A' + i))
		if p, exists := GetProcessID(wid); exists {
			if p.GetFlagExit() != 1 {
				t.Errorf("Process %s should be marked as killed", wid)
			}
		}
	}
}
