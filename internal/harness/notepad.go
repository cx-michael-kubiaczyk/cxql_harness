package harness

import (
	"fmt"
	"slices"
	"strings"
)

type Note struct {
	ID      string
	Type    string
	Content string
}

type Notepad struct {
	notes  []Note
	nextID int
}

func NewNotepad() *Notepad {
	return &Notepad{
		notes:  []Note{},
		nextID: 1,
	}
}

func (np *Notepad) Create(noteType, content string) {
	n := Note{ID: fmt.Sprintf("note-%04d", np.nextID), Type: noteType, Content: content}
	np.nextID++
	np.notes = append(np.notes, n)
	slices.SortFunc(np.notes, func(a, b Note) int {
		if a.Type == b.Type {
			return strings.Compare(a.ID, b.ID)
		} else {
			if a.Type == "Task" {
				return -1
			} else {
				return 1
			}
		}
	})
}

func (np *Notepad) Delete(id string) {
	for i, n := range np.notes {
		if n.ID == id {
			np.notes = append(np.notes[:i], np.notes[i+1:]...)
			return
		}
	}
}

func (np *Notepad) Update(id, content string) bool {
	for i, n := range np.notes {
		if n.ID == id {
			np.notes[i].Content = content
			return true
		}
	}
	return false
}

func (np *Notepad) All() []Note {
	return np.notes
}
