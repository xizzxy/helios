package store

// Nop (stub) store used in FAST mode when Redis is not compiled in.

type Client struct{}

func NewClientFromEnv() (*Client, error) { return &Client{}, nil }
func (c *Client) Close() error           { return nil }

// Compatibility types/aliases
type Stats map[string]any

func (c *Client) Stats() Stats    { return Stats{"mode": "nop"} }
func (c *Client) GetStats() Stats { return c.Stats() }
func (c *Client) Ping() error     { return nil }
