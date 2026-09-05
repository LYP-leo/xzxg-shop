package store

// Configure once, before serving requests. New stores fail closed: a simulated
// payment is only possible when the application explicitly enables demo mode.
func (s *MySQLStore) SetMockPaymentsEnabled(enabled bool) { s.mockPayments = enabled }

func (s *MySQLStore) MockPaymentsEnabled() bool { return s.mockPayments }
