package ui

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var frames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an animated spinner with a message on stderr.
// Call the returned function to stop and clear the spinner.
func Spinner(msg string) func() {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return func() {}
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	style := Renderer.NewStyle().Foreground(lipgloss.Color("245"))

	go func() {
		defer close(done)
		i := 0
		for {
			select {
			case <-stop:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			default:
				fmt.Fprintf(os.Stderr, "\r%s %s", style.Render(frames[i%len(frames)]), style.Render(msg))
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	return func() {
		close(stop)
		<-done
	}
}
