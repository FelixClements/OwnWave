package auth

import "context"

// UpdateUserProfile sets the user's email and full name; "" clears one.
// Normalize the values with NormalizeProfile first.
func (s *Service) UpdateUserProfile(ctx context.Context, userID, email, fullName string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users
		SET email = NULLIF($2, ''),
		    full_name = NULLIF($3, '')
		WHERE id = $1
	`, userID, email, fullName)
	return err
}

// CountUsers is the number of accounts.
func (s *Service) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}
