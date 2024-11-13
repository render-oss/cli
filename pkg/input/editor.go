package input

import (
	"os"

	"github.com/renderinc/cli/pkg/command"
)

func OpenEditorForInput(tmpFileName string, content string) (string, error) {
	file, err := os.CreateTemp("", tmpFileName)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(file.Name())
	}()

	_, err = file.WriteString(content)
	if err != nil {
		return "", err
	}

	editor := os.Getenv("EDITOR")

	err = command.RunProgram(editor, file.Name())
	if err != nil {
		return "", err
	}

	fileContent, err := os.ReadFile(file.Name())
	if err != nil {
		return "", err
	}

	return string(fileContent), nil
}
