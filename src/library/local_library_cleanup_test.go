package library

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestLocalLibraryCleanup inserts dangling albums and artists and checks that they
// are deleted as part of the clean-up.
func TestLocalLibraryCleanup(t *testing.T) {
	ctx := context.Background()

	lib, err := NewLocalLibrary(ctx, SQLiteMemoryFile, getTestMigrationFiles())
	if err != nil {
		t.Fatalf(err.Error())
	}
	defer func() { _ = lib.Truncate() }()

	if err := lib.Initialize(); err != nil {
		t.Fatalf("Initializing library: %s", err)
	}

	lib.fs = fstest.MapFS{}

	dbc := lib.db

	res, err := dbc.Exec(`
		INSERT INTO albums (name, fs_path)
		VALUES ('Lonely Album', '$1')
	`, filepath.FromSlash("/path/to/no/tracks"))
	if err != nil {
		t.Fatalf("error inserting album: %s", err)
	}
	albumID, _ := res.LastInsertId()

	res, err = dbc.Exec(`
		INSERT INTO artists (name)
		VALUES ('Fruitless Fellow')
	`)
	if err != nil {
		t.Fatalf("error inserting artist: %s", err)
	}
	artistID, _ := res.LastInsertId()

	stmt, err := dbc.Prepare(`
		INSERT INTO tracks (name, album_id, artist_id, number, fs_path, duration)
		VALUES
			('First Track', $1, $2, 1, $3, 100),
			('Second Track', $1, $2, 2, $4, 255)
	`)
	if err != nil {
		t.Fatalf("error preparing track insert: %s", err)
	}

	path1 := filepath.FromSlash("/does/not/exist/first.mp3")
	path2 := filepath.FromSlash("/does/not/exist/second.mp3")
	if _, err := stmt.Exec(albumID, artistID, path1, path2); err != nil {
		t.Fatalf("error inserting tracks: %s", err)
	}
	_ = stmt.Close()

	lib.cleanUpDatabase()

	rows, err := dbc.Query(`SELECT name FROM artists WHERE name = 'Fruitless Fellow'`)
	if err != nil {
		t.Fatalf("error querying for cleaned up artist: %s", err)
	}

	var foundArtists []string
	for rows.Next() {
		artist := ""
		if err := rows.Scan(&artist); err != nil {
			t.Errorf("error scanning database for artist: %s", err)
			continue
		}
		foundArtists = append(foundArtists, artist)
	}

	if len(foundArtists) > 0 {
		t.Errorf("expected dangling artist to have been cleaned up but it was not")
	}

	rows, err = dbc.Query(`SELECT name FROM albums WHERE name = 'Lonely Album'`)
	if err != nil {
		t.Fatalf("error querying for cleaned up album: %s", err)
	}

	var foundAlbums []string
	for rows.Next() {
		album := ""
		if err := rows.Scan(&album); err != nil {
			t.Errorf("error scanning database for album: %s", err)
			continue
		}
		foundAlbums = append(foundAlbums, album)
	}

	if len(foundAlbums) > 0 {
		t.Errorf("expected dangling albums to have been cleaned up but they were not")
	}
}
