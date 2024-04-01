package common

import (
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/schollz/progressbar/v3"
)

type (
	// DummyProgressReporter is just a stub.
	DummyProgressReporter struct {
	}

	// MultiProgressbarReporter reportes progress of individual taks via progressbars.
	MultiProgressbarReporter struct {
		active   []int
		bars     map[int]*progressbar.ProgressBar
		barLock  sync.Mutex
		wrLock   sync.Mutex
		output   io.Writer
		curLine  int
		totLines int
	}

	// io.Writer wrapper to know which progressbar wants to write.
	multiProgressbarWriter struct {
		*MultiProgressbarReporter
		Index int
	}

	// ProgressReporter is a generic interface to setting up progress reporting.
	// Currently, it provisions following methods:
	//   * AdvanceTask advance progress on a task given by its index by a cetain amount.
	//   * CreateFileTask creates a new file task and return its index.
	//   * DescribeTask change task description given by its index.
	//   * FinishTask finish a task given by its index.
	ProgressReporter interface {
		AdvanceTask(int, int64) error
		CreateFileTask(int64) (int, error)
		DescribeTask(int, string) error
		FinishTask(int) error
	}
)

// AdvanceTask stub.
func (dummy *DummyProgressReporter) AdvanceTask(index int, increment int64) error {
	return nil
}

// CreateFileTask stub.
func (dummy *DummyProgressReporter) CreateFileTask(size int64) (int, error) {
	return 0, nil
}

// DescribeTask stub.
func (dummy *DummyProgressReporter) DescribeTask(index int, description string) error {
	return nil
}

// FinishTask stub.
func (dummy *DummyProgressReporter) FinishTask(index int) error {
	return nil
}

// NewMultiProgressbarReporter creates a MultiProgressBarReporter with a given `output`.
func NewMultiProgressbarReporter(output io.Writer) *MultiProgressbarReporter {
	return &MultiProgressbarReporter{
		active:   []int{},
		bars:     map[int]*progressbar.ProgressBar{},
		barLock:  sync.Mutex{},
		wrLock:   sync.Mutex{},
		output:   output,
		curLine:  1,
		totLines: 1,
	}
}

// AdvanceTask advances the progress by `increment` on a task specified by `index`.
func (mpr *MultiProgressbarReporter) AdvanceTask(index int, increment int64) error {
	mpr.barLock.Lock()
	defer mpr.barLock.Unlock()

	if _, prs := mpr.bars[index]; !prs {
		return fmt.Errorf("task index %d outside of available range", index)
	}

	if !mpr.bars[index].IsFinished() {
		if !slices.Contains(mpr.active, index) {
			mpr.active = append(mpr.active, index)

			progressbar.OptionSetWriter(&multiProgressbarWriter{
				MultiProgressbarReporter: mpr,
				Index:                    len(mpr.active),
			})(mpr.bars[index])

			if len(mpr.active) > mpr.totLines {
				fmt.Fprintf(mpr.output, "\n")
				mpr.totLines++
			}
		}

		_ = mpr.bars[index].Add64(increment)

		if mpr.bars[index].IsFinished() {
			mpr.remove(index)
		}
	}

	return nil
}

// CreateFileTask will create an invisible progressbar for a file task. It will be displayed
// upon first call to AdvanceTask.
func (mpr *MultiProgressbarReporter) CreateFileTask(size int64) (int, error) {
	mpr.barLock.Lock()
	defer mpr.barLock.Unlock()

	var index int = len(mpr.bars) + 1

	var bar *progressbar.ProgressBar = progressbar.NewOptions64(
		size,
		progressbar.OptionSetWriter(io.Discard),
		progressbar.OptionShowBytes(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetPredictTime(false),
		progressbar.OptionSetRenderBlankState(false),
	)

	mpr.bars[index] = bar

	return index, nil
}

// DescribeTask set desription of a task specified by `index`.
func (mpr *MultiProgressbarReporter) DescribeTask(index int, description string) error {
	mpr.barLock.Lock()
	defer mpr.barLock.Unlock()

	if _, prs := mpr.bars[index]; !prs {
		return fmt.Errorf("task index %d outside of available range", index)
	}

	mpr.bars[index].Describe(description)

	return nil
}

// FinishTask finish task specified by `index`.
func (mpr *MultiProgressbarReporter) FinishTask(index int) error {
	mpr.barLock.Lock()
	defer mpr.barLock.Unlock()

	if _, prs := mpr.bars[index]; !prs {
		return fmt.Errorf("task index %d outside of available range", index)
	}

	_ = mpr.bars[index].Finish()

	if mpr.bars[index].IsFinished() {
		mpr.remove(index)
	}

	return nil
}

// remove a task that has finished
func (mpr *MultiProgressbarReporter) remove(index int) {
	active := slices.Index(mpr.active, index)

	mpr.active[active] = mpr.active[len(mpr.active)-1]
	mpr.active = mpr.active[:len(mpr.active)-1]

	_ = mpr.bars[index].Clear()
	progressbar.OptionSetWriter(io.Discard)(mpr.bars[index])

	if len(mpr.active) > 0 && len(mpr.active) != active {
		_ = mpr.bars[mpr.active[active]].Clear()
		progressbar.OptionSetWriter(&multiProgressbarWriter{
			MultiProgressbarReporter: mpr,
			Index:                    active + 1,
		})(mpr.bars[mpr.active[active]])
		_ = mpr.bars[mpr.active[active]].RenderBlank()
	}
	delete(mpr.bars, index)
}

// Write to output stream on the respective line.
func (mpw *multiProgressbarWriter) Write(p []byte) (n int, err error) {
	mpw.wrLock.Lock()
	defer mpw.wrLock.Unlock()

	n, err = mpw.move(mpw.Index, mpw.output)
	if err != nil {
		return n, err
	}
	return mpw.output.Write(p)
}

// Move cursor to the beginning of the current progressbar.
func (mpw *multiProgressbarWriter) move(index int, writer io.Writer) (int, error) {
	bias := mpw.curLine - index
	mpw.curLine = index
	if bias > 0 {
		// move up
		return fmt.Fprintf(writer, "\r\033[%dA", bias)
	} else if bias < 0 {
		// move down
		return fmt.Fprintf(writer, "\r\033[%dB", -bias)
	}
	return 0, nil
}
