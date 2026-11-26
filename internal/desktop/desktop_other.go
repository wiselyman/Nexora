//go:build !darwin && !linux && !windows

package desktop

type unsupported struct{}

func New() Adapter { return unsupported{} }
func (unsupported) Info() map[string]any {
	return map[string]any{"desktop_supported": false, "note": "当前操作系统未提供 Nexora 桌面控制适配器。"}
}
func (unsupported) Screenshot() ([]byte, error)        { return nil, errUnsupported }
func (unsupported) Windows() ([]map[string]any, error) { return nil, errUnsupported }
func (unsupported) Pointer(int, int, int) error        { return errUnsupported }
func (unsupported) Key(string) error                   { return errUnsupported }
func (unsupported) Type(string) error                  { return errUnsupported }
