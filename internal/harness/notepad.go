package harness

import "fmt"

type Note struct {
	ID      string
	Type    string
	Content string
}

type Notepad struct {
	notes  []Note
	nextID int
}

func (np *Notepad) Create(noteType, content string) Note {
	np.nextID++
	n := Note{ID: fmt.Sprintf("note-%d", np.nextID), Type: noteType, Content: content}
	np.notes = append(np.notes, n)
	return n
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
