package domain

import (
	"database/sql/driver"
	"fmt"
	"math/rand"
	"strings"
)

const base58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

type ID interface {
	fmt.Stringer
	Kind() Kind
}

func GenerateRandomStringFromAlphabet(size int, alphabet string) string {
	buf := make([]byte, size)
	for i := 0; i < size; i++ {
		buf[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(buf)
}

// ConvertTfeID converts an ID for use with a different resource kind, e.g. convert
// run-123 to plan-123.
func ConvertTfeID(id TfeID, to Kind) TfeID {
	return TfeID{kind: to, id: id.id}
}

func MustHardcodeTfeID(kind Kind, suffix string) TfeID {
	s := fmt.Sprintf("%s-%s", kind, suffix)
	id, err := ParseTfeID(s)
	if err != nil {
		panic("failed to parse hardcoded ID: " + err.Error())
	}
	return id
}

// ParseTfeID parses the ID from a string representation.
func ParseTfeID(s string) (TfeID, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return TfeID{}, fmt.Errorf("malformed ID: %s", s)
	}
	kind := parts[0]
	if len(kind) < 2 {
		return TfeID{}, fmt.Errorf("kind must be at least 2 characters: %s", s)
	}
	id := parts[1]
	if len(id) < 1 {
		return TfeID{}, fmt.Errorf("id suffix must be at least 1 character: %s", s)
	}
	return TfeID{kind: Kind(kind), id: id}, nil
}

type TfeID struct {
	kind Kind
	id   string
}

func (id TfeID) String() string {
	return fmt.Sprintf("%s-%s", id.kind, id.id)
}

func (id TfeID) Kind() Kind {
	return id.kind
}

func (id TfeID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}

func (id *TfeID) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return nil
	}
	s := string(text)
	x, err := ParseTfeID(s)
	if err != nil {
		return err
	}
	*id = x
	return nil
}

func (id *TfeID) Scan(text any) error {
	if text == nil {
		return nil
	}
	s, ok := text.(string)
	if !ok {
		return fmt.Errorf("expected database value to be a string: %#v", text)
	}
	x, err := ParseTfeID(s)
	if err != nil {
		return err
	}
	*id = x
	return nil
}

func (id *TfeID) Value() (driver.Value, error) {
	if id == nil {
		return nil, nil
	}
	return id.String(), nil
}

func NewTfeID(kind Kind) TfeID {
	return TfeID{kind: kind, id: GenerateRandomStringFromAlphabet(16, base58)}
}

func NewTfeIDWithVal(kind Kind, id string) TfeID {
	return TfeID{kind: kind, id: id}
}
