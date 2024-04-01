package common

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

type failWriter struct {
}

func (f *failWriter) Write(p []byte) (int, error) {
	return len(p), fmt.Errorf("failWriter write failed")
}

/* test cases for MultiProgressbarReporter FileTask */
func ExampleMultiProgressbarReporter() {
	var index int

	// Setup Test
	mockMPR := NewMultiProgressbarReporter(os.Stdout)
	index, _ = mockMPR.CreateFileTask(100)
	_ = mockMPR.DescribeTask(index, "test")

	// Simulate some work
	time.Sleep(1 * time.Second)
	_ = mockMPR.AdvanceTask(index, 10)

	// Output:
	// test  10% |████                                    | (10 B/s)
}

func ExampleMultiProgressbarReporter_spinner() {
	var index int

	// Setup Test
	mockMPR := NewMultiProgressbarReporter(os.Stdout)
	index, _ = mockMPR.CreateFileTask(-1)
	_ = mockMPR.DescribeTask(index, "test")

	// Simulate some work
	time.Sleep(1 * time.Second)
	_ = mockMPR.AdvanceTask(index, 10)

	// Output:
	// - test (10 B/s) [1s]
}

func TestFinishFileTask(t *testing.T) {
	var index int
	var err error
	var output bytes.Buffer
	outputWriter := io.Writer(&output)

	// Setup Test
	mockMPR := NewMultiProgressbarReporter(outputWriter)

	index, err = mockMPR.CreateFileTask(-1)
	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}
	err = mockMPR.DescribeTask(index, "test ")
	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}

	// Simulate some work
	fmt.Fprintf(outputWriter, "some description presceeding the work cycle\n")
	err = mockMPR.AdvanceTask(index, 100)
	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}

	// Finish the task
	err = mockMPR.FinishTask(index)
	if err != nil {
		t.Fatalf("unexpected test result: %+v", err)
	}

	assertEquals(t, "some description presceeding the work cycle\n", output.String()[:44], "TestFinishFileTask.Output")
}

func TestConcurrentFileTasks(t *testing.T) {
	var index []int = []int{0, 0, 0, 0, 0}
	var ubound []int = []int{300, 100, 50, 200, 150}
	var err error
	var output bytes.Buffer
	outputWriter := io.Writer(&output)

	// Setup Test
	mockMPR := NewMultiProgressbarReporter(outputWriter)

	for i := range index {
		index[i], err = mockMPR.CreateFileTask(int64(ubound[i]))
		if err != nil {
			t.Fatalf("unexpected test result: %+v, %+v", i, err)
		}
		err = mockMPR.DescribeTask(index[i], fmt.Sprintf("task #%d", i))
		if err != nil {
			t.Fatalf("unexpected test result: %+v, %+v", i, err)
		}
	}

	// Simulate some work
	fmt.Fprintf(outputWriter, "some description presceeding the work cycle\n")
	var total int = 0
	for i := 0; i < 6; i++ {
		total += 50
		for j := range index {
			err = mockMPR.AdvanceTask(index[j], 50)
			if total > ubound[j] {
				assertEquals(t, fmt.Sprintf("task index %d outside of available range", index[j]), err.Error(), "TestConcurrentFileTasks.Error")
			} else {
				if err != nil {
					t.Fatalf("unexpected test result: %+v, %+v", j, err)
				}
			}

		}
	}

	assertEquals(t, "some description presceeding the work cycle\n", output.String()[:44], "TestConcurrentFileTasks.Output")
}

func TestInvalidFileTask(t *testing.T) {
	var index int
	var err error
	var output bytes.Buffer
	outputWriter := io.Writer(&output)

	// Setup Test
	mockMPR := NewMultiProgressbarReporter(outputWriter)
	index, err = mockMPR.CreateFileTask(100)
	if err != nil {
		t.Fatalf("CreateFileTask: unexpected test result: %+v, %+v", index, err)
	}

	// Simulate some work
	err = mockMPR.AdvanceTask(index, 10)
	if err != nil {
		t.Fatalf("AdvanceTask: unexpected test result: %+v, %+v", index, err)
	}
	err = mockMPR.DescribeTask(index, "valid task")
	if err != nil {
		t.Fatalf("DescribeTask: unexpected test result: %+v, %+v", index, err)
	}

	// Call with invalid index
	err = mockMPR.AdvanceTask(-1, 10)
	if err == nil {
		t.Fatalf("AdvanceTask: unexpected test result: -1, %+v", err)
	} else {
		assertEquals(t, "task index -1 outside of available range", err.Error(), "TestInvalidFileTask.Error")
	}

	// Call with invalid index
	err = mockMPR.DescribeTask(2, "invalid")
	if err == nil {
		t.Fatalf("DescribeTask, unexpected test result: 2, %+v", err)
	} else {
		assertEquals(t, "task index 2 outside of available range", err.Error(), "TestInvalidFileTask.Error")
	}

	// Call with invalid index
	err = mockMPR.FinishTask(3)
	if err == nil {
		t.Fatalf("FinishTask, unexpected test result: 3, %+v", err)
	} else {
		assertEquals(t, "task index 3 outside of available range", err.Error(), "TestInvalidFileTask.Error")
	}
}

func TestMultiProgressbarWriterInvalidMove(t *testing.T) {
	// Setup the test
	mockMPR := &MultiProgressbarReporter{
		active:  []int{},
		bars:    nil,
		barLock: sync.Mutex{},
		wrLock:  sync.Mutex{},
		output:  &failWriter{},
		curLine: -1,
	}
	mockWriter := &multiProgressbarWriter{
		mockMPR,
		0,
	}

	// Perform the test
	data := []byte("test")
	n, err := mockWriter.Write(data)

	if err == nil {
		t.Fatalf("Write, unexpected test result: %d, %+v", n, err)
	} else {
		assertEquals(t, "failWriter write failed", err.Error(), "TestMultiProgressbarWriterInvalidMove.Error")
	}
}
