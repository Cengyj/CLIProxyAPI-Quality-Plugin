package quality

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

type Preset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Config    RunConfig `json:"config"`
	CreatedAt time.Time `json:"created_at"`
}

func (en *Engine) Presets() ([]Preset, error) {
	out := []Preset{}
	err := en.store.All("presets", func(b []byte) error {
		var p Preset
		if e := json.Unmarshal(b, &p); e != nil {
			return e
		}
		out = append(out, p)
		return nil
	})
	return out, err
}
func (en *Engine) SavePreset(name string, c RunConfig) (Preset, error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	if err := en.mutableLocked(); err != nil {
		return Preset{}, err
	}
	if name == "" || len(name) > 128 {
		return Preset{}, fmt.Errorf("preset name requires 1..128 bytes")
	}
	if err := en.validateManual(&c); err != nil {
		return Preset{}, err
	}
	if _, err := FreezeQuestion(c); err != nil {
		return Preset{}, err
	}
	if err := en.storageAdmission(); err != nil {
		return Preset{}, err
	}
	c = cloneConfig(c)
	c.IdempotencyKey = ""
	p := Preset{ID: ID(), Name: name, Config: c, CreatedAt: en.now()}
	return p, en.store.Put("presets", p.ID, p)
}
func (en *Engine) DeletePreset(id string) error {
	en.mu.Lock()
	defer en.mu.Unlock()
	if err := en.mutableLocked(); err != nil {
		return err
	}
	return en.store.Write(func(tx *bolt.Tx) error { return tx.Bucket([]byte("presets")).Delete([]byte(id)) })
}
