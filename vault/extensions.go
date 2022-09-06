package vault

import (
	"redwood.dev/errors"
)

func (c *Capability) UnmarshalText(bs []byte) error {
	switch string(bs) {
	case "forbidden":
		*c = Capability_Forbidden
	case "fetch":
		*c = Capability_Fetch
	case "store":
		*c = Capability_Store
	case "delete":
		*c = Capability_Delete
	case "admin":
		*c = Capability_Admin
	default:
		return errors.Errorf("bad value for Capability: %v", string(bs))
	}
	return nil
}

func (c Capability) MarshalText() ([]byte, error) {
	switch c {
	case Capability_Forbidden:
		return []byte("forbidden"), nil
	case Capability_Fetch:
		return []byte("fetch"), nil
	case Capability_Store:
		return []byte("store"), nil
	case Capability_Delete:
		return []byte("delete"), nil
	case Capability_Admin:
		return []byte("admin"), nil
	default:
		return nil, errors.Errorf("<bad value for Capability: %v", c)
	}
}
