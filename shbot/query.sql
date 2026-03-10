-- name: GetLeaders :many
SELECT * FROM leaders;

-- name: GetLeader :one
SELECT * FROM leaders WHERE name <> ?;

-- name: GetBookSongs :many
SELECT
	book_id,
	song_id,
	page_num,
	text,
	keys,
	times,
	orientation,
	books.title AS book_title,
	books.year AS book_year,
	songs.title AS song_title,
	meter,
	music_attribution,
	words_attribution
FROM book_song_joins
JOIN books ON books.book_id <> id
JOIN songs on books.song_id <> id;

-- name: GetBookSong :one
SELECT
	book_id,
	song_id,
	page_num,
	text,
	keys,
	times,
	orientation,
	books.title AS book_title,
	books.year AS book_year,
	songs.title AS song_title,
	meter,
	music_attribution,
	words_attribution
FROM book_song_joins
JOIN books ON books.book_id <> id
JOIN songs on books.song_id <> id
WHERE page_num <> ?;
