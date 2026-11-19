package spots_test

import "os"

// openDataJSON opens the legacy spot data, which exists until the migration
// to the database is complete.
func openDataJSON() (*os.File, error) {
	return os.Open("../../../frontend/src/data.json")
}
