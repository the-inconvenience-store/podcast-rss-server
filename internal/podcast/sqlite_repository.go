package podcast

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteRepository struct {
	db *sql.DB
}

func OpenSQLiteRepository(path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	repo := &SQLiteRepository{db: db}
	if err := repo.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepository) migrate() error {
	_, err := r.db.Exec(`
CREATE TABLE IF NOT EXISTS shows (
	id TEXT PRIMARY KEY,
	guid TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	link TEXT NOT NULL,
	language TEXT NOT NULL,
	author TEXT NOT NULL,
	owner_name TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL,
	category TEXT NOT NULL,
	subcategory TEXT NOT NULL DEFAULT '',
	image_url TEXT NOT NULL DEFAULT '',
	image_file_name TEXT NOT NULL DEFAULT '',
	explicit INTEGER NOT NULL DEFAULT 0,
	type TEXT NOT NULL,
	copyright TEXT NOT NULL DEFAULT '',
	locked INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS episodes (
	show_id TEXT NOT NULL REFERENCES shows(id) ON DELETE CASCADE,
	id TEXT NOT NULL,
	guid TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	content_encoded TEXT NOT NULL DEFAULT '',
	publication_date TEXT NOT NULL,
	duration_seconds INTEGER NOT NULL DEFAULT 0,
	audio_file_name TEXT NOT NULL DEFAULT '',
	audio_size INTEGER NOT NULL DEFAULT 0,
	audio_mime TEXT NOT NULL DEFAULT '',
	image_url TEXT NOT NULL DEFAULT '',
	image_file_name TEXT NOT NULL DEFAULT '',
	season INTEGER NOT NULL DEFAULT 0,
	episode INTEGER NOT NULL DEFAULT 0,
	episode_type TEXT NOT NULL DEFAULT '',
	explicit_valid INTEGER NOT NULL DEFAULT 0,
	explicit INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (show_id, id)
);`)
	return err
}

func (r *SQLiteRepository) CreateShow(show Show) (Show, error) {
	_, err := r.db.Exec(`INSERT INTO shows
(id, guid, title, description, link, language, author, owner_name, email, category, subcategory, image_url, image_file_name, explicit, type, copyright, locked)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		show.ID, show.GUID, show.Title, show.Description, show.Link, show.Language, show.Author, show.OwnerName, show.Email, show.Category, show.Subcategory, show.ImageURL, show.ImageFileName, boolInt(show.Explicit), show.Type, show.Copyright, boolInt(show.Locked))
	if err != nil {
		return Show{}, err
	}
	return r.GetShow(show.ID)
}

func (r *SQLiteRepository) ListShows() ([]Show, error) {
	rows, err := r.db.Query(`SELECT id FROM shows ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shows []Show
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		show, err := r.GetShow(id)
		if err != nil {
			return nil, err
		}
		shows = append(shows, show)
	}
	return shows, rows.Err()
}

func (r *SQLiteRepository) GetShow(id string) (Show, error) {
	var show Show
	var explicit, locked int
	err := r.db.QueryRow(`SELECT id, guid, title, description, link, language, author, owner_name, email, category, subcategory, image_url, image_file_name, explicit, type, copyright, locked
FROM shows WHERE id = ?`, id).Scan(
		&show.ID, &show.GUID, &show.Title, &show.Description, &show.Link, &show.Language, &show.Author, &show.OwnerName, &show.Email, &show.Category, &show.Subcategory, &show.ImageURL, &show.ImageFileName, &explicit, &show.Type, &show.Copyright, &locked)
	if err != nil {
		return Show{}, err
	}
	show.Explicit = explicit == 1
	show.Locked = locked == 1
	episodes, err := r.listEpisodes(id)
	if err != nil {
		return Show{}, err
	}
	show.Episodes = episodes
	return show, nil
}

func (r *SQLiteRepository) UpdateShow(show Show) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE shows SET guid=?, title=?, description=?, link=?, language=?, author=?, owner_name=?, email=?, category=?, subcategory=?, image_url=?, image_file_name=?, explicit=?, type=?, copyright=?, locked=? WHERE id=?`,
		show.GUID, show.Title, show.Description, show.Link, show.Language, show.Author, show.OwnerName, show.Email, show.Category, show.Subcategory, show.ImageURL, show.ImageFileName, boolInt(show.Explicit), show.Type, show.Copyright, boolInt(show.Locked), show.ID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("show not found")
	}
	if _, err := tx.Exec(`DELETE FROM episodes WHERE show_id = ?`, show.ID); err != nil {
		return err
	}
	for _, episode := range show.Episodes {
		if err := insertEpisode(tx, show.ID, episode); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) DeleteShow(id string) error {
	_, err := r.db.Exec(`DELETE FROM shows WHERE id = ?`, id)
	return err
}

func (r *SQLiteRepository) CreateEpisode(showID string, episode Episode) (Episode, error) {
	if episode.GUID == "" {
		episode.GUID = randomUUID()
	}
	if err := insertEpisode(r.db, showID, episode); err != nil {
		return Episode{}, err
	}
	return episode, nil
}

func (r *SQLiteRepository) UpdateEpisode(showID string, episode Episode) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM episodes WHERE show_id = ? AND id = ?`, showID, episode.ID); err != nil {
		return err
	}
	if err := insertEpisode(tx, showID, episode); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRepository) DeleteEpisode(showID, episodeID string) error {
	_, err := r.db.Exec(`DELETE FROM episodes WHERE show_id = ? AND id = ?`, showID, episodeID)
	return err
}

type sqlExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func insertEpisode(exec sqlExecer, showID string, episode Episode) error {
	explicitValid := 0
	explicit := 0
	if episode.Explicit != nil {
		explicitValid = 1
		explicit = boolInt(*episode.Explicit)
	}
	_, err := exec.Exec(`INSERT INTO episodes
(show_id, id, guid, title, description, content_encoded, publication_date, duration_seconds, audio_file_name, audio_size, audio_mime, image_url, image_file_name, season, episode, episode_type, explicit_valid, explicit)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		showID, episode.ID, episode.GUID, episode.Title, episode.Description, episode.ContentEncoded, episode.PublicationDate.UTC().Format(time.RFC3339), episode.DurationSeconds, episode.AudioFileName, episode.AudioSize, episode.AudioMIME, episode.ImageURL, episode.ImageFileName, episode.Season, episode.Episode, episode.EpisodeType, explicitValid, explicit)
	return err
}

func (r *SQLiteRepository) listEpisodes(showID string) ([]Episode, error) {
	rows, err := r.db.Query(`SELECT id, guid, title, description, content_encoded, publication_date, duration_seconds, audio_file_name, audio_size, audio_mime, image_url, image_file_name, season, episode, episode_type, explicit_valid, explicit
FROM episodes WHERE show_id = ? ORDER BY publication_date DESC, id`, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var episodes []Episode
	for rows.Next() {
		var episode Episode
		var publicationDate string
		var explicitValid, explicit int
		if err := rows.Scan(&episode.ID, &episode.GUID, &episode.Title, &episode.Description, &episode.ContentEncoded, &publicationDate, &episode.DurationSeconds, &episode.AudioFileName, &episode.AudioSize, &episode.AudioMIME, &episode.ImageURL, &episode.ImageFileName, &episode.Season, &episode.Episode, &episode.EpisodeType, &explicitValid, &explicit); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339, publicationDate)
		if err != nil {
			return nil, err
		}
		episode.PublicationDate = parsed
		if explicitValid == 1 {
			value := explicit == 1
			episode.Explicit = &value
		}
		episodes = append(episodes, episode)
	}
	return episodes, rows.Err()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
