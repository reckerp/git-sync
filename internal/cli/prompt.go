package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type prompter struct {
	in  *bufio.Reader
	out io.Writer
}

func newPrompter(in io.Reader, out io.Writer) prompter {
	return prompter{in: bufio.NewReader(in), out: out}
}

// ask prints question and returns the trimmed answer line.
func (p prompter) ask(question string) (string, error) {
	fmt.Fprint(p.out, question)
	line, err := p.in.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || line == "") {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// confirm asks a yes/no question that defaults to yes.
func (p prompter) confirm(question string) (bool, error) {
	answer, err := p.ask(question + " [Y/n] ")
	if err != nil {
		return false, err
	}
	return answer == "" || strings.HasPrefix(strings.ToLower(answer), "y"), nil
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
