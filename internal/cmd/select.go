package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// multiselect displays an interactive checkbox list with keyboard navigation.
// ↑/↓ to move, Space to toggle, a to toggle all, Enter to confirm.
func multiselect(items []string) ([]int, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to select")
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return multiselectText(items)
	}

	selected := make([]bool, len(items))
	cursor := 0
	linesPrinted := 0

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return multiselectText(items)
	}
	defer term.Restore(fd, oldState)

	render := func() {
		if linesPrinted > 0 {
			fmt.Printf("\033[%dA\033[J", linesPrinted)
		}
		linesPrinted = 0

		for i, item := range items {
			if i == cursor {
				if selected[i] {
					fmt.Printf("\033[36m> [x] %s\033[0m\r\n", item)
				} else {
					fmt.Printf("\033[36m> [ ] %s\033[0m\r\n", item)
				}
			} else {
				if selected[i] {
					fmt.Printf("  \033[32m[x]\033[0m %s\r\n", item)
				} else {
					fmt.Printf("  [ ] %s\r\n", item)
				}
			}
			linesPrinted++
		}
		fmt.Printf("\033[2m  ↑↓ move  space toggle  a select all  enter confirm\033[0m\r\n")
		linesPrinted++
	}

	render()

	buf := make([]byte, 3)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			continue
		}

		switch {
		case buf[0] == '\r' || buf[0] == '\n':
			goto done
		case buf[0] == 0x1b || buf[0] == 'q':
			if n == 1 {
				return nil, fmt.Errorf("cancelled")
			}
			if n >= 3 && buf[1] == '[' {
				switch buf[2] {
				case 'A':
					if cursor > 0 {
						cursor--
					}
				case 'B':
					if cursor < len(items)-1 {
						cursor++
					}
				}
			}
		case buf[0] == ' ':
			selected[cursor] = !selected[cursor]
			if cursor < len(items)-1 {
				cursor++
			}
		case buf[0] == 'k':
			if cursor > 0 {
				cursor--
			}
		case buf[0] == 'j':
			if cursor < len(items)-1 {
				cursor++
			}
		case buf[0] == 'a':
			allSelected := true
			for _, s := range selected {
				if !s {
					allSelected = false
					break
				}
			}
			for i := range selected {
				selected[i] = !allSelected
			}
		}

		render()
	}

done:
	var result []int
	for i, s := range selected {
		if s {
			result = append(result, i)
		}
	}
	return result, nil
}

// multiselectText is the fallback for non-TTY stdin.
// Uses text input: numbers to toggle, Enter to confirm.
func multiselectText(items []string) ([]int, error) {
	selected := make([]bool, len(items))

	reader := bufio.NewReader(os.Stdin)
	linesPrinted := 0

	render := func() {
		if linesPrinted > 0 {
			fmt.Printf("\033[%dA\033[J", linesPrinted)
		}
		linesPrinted = 0

		for i, item := range items {
			mark := " "
			if selected[i] {
				mark = "x"
			}
			fmt.Printf("  [%s] %d. %s\n", mark, i+1, item)
			linesPrinted++
		}
		fmt.Printf("\033[2m  type numbers to toggle, a select all, enter confirm\033[0m\n")
		linesPrinted++
	}

	render()

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			break
		}

		if input == "a" || input == "all" {
			allSelected := true
			for _, s := range selected {
				if !s {
					allSelected = false
					break
				}
			}
			for i := range selected {
				selected[i] = !allSelected
			}
			render()
			continue
		}

		valid := true
		for _, part := range strings.Split(input, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			if strings.Contains(part, "-") {
				bounds := strings.SplitN(part, "-", 2)
				start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
				end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
				if err1 != nil || err2 != nil || start < 1 || end < start || end > len(items) {
					valid = false
					break
				}
				for j := start - 1; j < end; j++ {
					selected[j] = !selected[j]
				}
				continue
			}

			idx, err := strconv.Atoi(part)
			if err != nil || idx < 1 || idx > len(items) {
				valid = false
				break
			}
			selected[idx-1] = !selected[idx-1]
		}

		if !valid {
			render()
			continue
		}

		render()
	}

	var result []int
	for i, s := range selected {
		if s {
			result = append(result, i)
		}
	}
	return result, nil
}
