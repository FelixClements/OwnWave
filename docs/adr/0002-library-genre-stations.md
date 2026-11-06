# 0002: Stations play the whole library

Status: accepted, 2026-11-05. Migrations 023 and 024. Partly supersedes
[ADR 0001](0001-auto-genre-stations.md): decisions 1 and 2 for genre stations,
the counts in decision 4, and decision 6.

## Context

The library held 6290 files, about 4390 songs, and only about 800 files were
matched to a playlist. Stations played matches only, so most of the library
was never heard. Going to the whole library exposes three problems that
matches hid: wrong genres reach listeners (the audio model alone agrees with
60% of the file tags we can check, and only 25% for techno), duplicate files
show up (about 1900 extra copies, some disagreeing on the main genre), and a
plain shuffle gives a large pool no guarantee that every song gets a turn.

Listeners should not hear playlist songs more often than the rest. A song may
belong to several playlists.

## Decision

1. **A genre station plays every song of the library with that main genre.**
   It has no playlist pool: migration 023 sets `all_playlists = FALSE` on genre
   stations, and `Definition.Library` follows from the station's kind. Station
   ids, names, history and feedback stay. Main genre only (`is_primary`): second
   genres would put a song on up to three stations. Thresholds count songs the
   same way, so fewer stations may qualify than before for a genre that lived
   on second genres, and more appear for a genre with few matches.

2. **Copies are tracked, never removed.** `tracks.copy_of` points a file at its
   song's best copy, and is NULL on best copies and unique files. Files with
   the same normalised title and version, a shared artist and the same length
   within 2 seconds are one song. The best copy is the one the matcher prefers:
   highest sample rate, then largest file, then lowest id. Grouping runs at the
   start of a background pipeline run and only writes this column. Album, track
   and library views keep showing every file. Files missing a title, an artist
   or a length are never grouped: a wrong merge would lose a song from stations,
   and the queue's existing title and artist dedupe still catches most of them.

3. **Stations work on songs.** Every pool holds best copies only. A ban, like,
   hide or play of any copy counts for the song.

4. **"Not in this station".** `station_hidden_tracks` lets a user take a song off
   one station for themselves. It is the light answer to a wrong genre; a ban
   still means never anywhere. The station page lists hidden songs to bring back.

5. **Genre corrections.** `genre_corrections` holds an admin's genre for a song or
   an artist. Scoring in the worker applies it last, so a rescan or rescore keeps
   it: song over artist over blend, and the corrected row is marked `corrected`.
   An artist correction matches the whole credit or its first artist. It lives
   outside `track_station_genres` because scoring rewrites that table. The admin
   review list ranks songs by how many listeners hid them on a genre station,
   then by copies disagreeing, then by lowest score. Saving the current genre
   ("Looks right") takes a song off the list.

6. **The queue gives every song a turn.** Never played on this station comes
   first, shuffled. Then songs played longest ago, in shuffled week-wide groups.
   Last, songs played within `STATION_RECENT_HOURS`, oldest first. Without this a
   pool of about 900 songs in a 200-song page would repeat at random.

7. **Library picks extend playlists.** `playlist_library_picks` holds library
   songs that sound like a playlist. Each matched song offers its 5 nearest best
   copies by embedding cosine, at similarity 0.7 or more. Candidates rank by how
   many of the playlist's songs offered them, then by best similarity. A playlist
   gains at most as many picks as it has matched songs, so matches stay at least
   half of the station, and every song has the same chance. A song matched by
   another playlist can be a pick here. All picks are on by default with no per
   playlist switch. An admin can remove a pick for good
   (`playlist_pick_removals`).

8. **Rebuilding picks.** `playlists.picks_input` stores the rule version, a library
   signature and a hash of the playlist's songs; unchanged input means no rebuild.
   A new match rebuilds that playlist in the request. A library change rebuilds
   every playlist, which measured about 40 seconds of database time on the live
   library, so only background runs do it. Searches are exact scans: at this size
   no vector index is needed.

## Consequences

- A genre station can no longer collide with a custom station: "Techno, every
  playlist" and the Techno genre station play different songs. The 409 rules
  from ADR 0001 are gone, and suggestions no longer skip a genre for that reason.
- Deleting a playlist no longer changes a genre station.
- Wrong main genres now reach listeners. Hides and the review list are how they
  get found and fixed. Artists with a single sound are the quickest fix.
- Hides, bans, likes and song corrections are stored on one file. Deleting that
  file from disk drops them, even if another copy remains.
- Same title, artist and version with a length more than 2 seconds apart stay two
  songs. Picks still skip them when the title and an artist are equal.
- Picks drift: a playlist with few matches or a broad sound gets some songs that
  only fit by audio. Removing one is one click for an admin.
- Deleting copies to save disk space is a separate decision; `copy_of` is what
  such a report would use.
