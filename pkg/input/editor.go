package input

import (
	"os"
	"os/exec"
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

	editorCMD := exec.Command(editor, file.Name())
	editorCMD.Stdin = os.Stdin
	editorCMD.Stdout = os.Stdout
	editorCMD.Stderr = os.Stderr

	err = editorCMD.Run()
	if err != nil {
		return "", err
	}

	fileContent, err := os.ReadFile(file.Name())
	if err != nil {
		return "", err
	}

	return string(fileContent), nil
}
