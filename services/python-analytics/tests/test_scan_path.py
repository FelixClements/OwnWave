import pytest
from fastapi import HTTPException

def test_resolved_scan_path_default(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    music_dir.mkdir()
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    resolved = api._resolved_scan_path(str(music_dir))
    assert resolved == str(music_dir.resolve())

def test_resolved_scan_path_relative(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    rock_dir = music_dir / "rock"
    rock_dir.mkdir(parents=True)
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    resolved = api._resolved_scan_path("rock")
    assert resolved == str(rock_dir.resolve())

def test_resolved_scan_path_traversal(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    music_dir.mkdir()
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    with pytest.raises(HTTPException) as exc_info:
        api._resolved_scan_path("../../etc/passwd")
    assert exc_info.value.status_code == 400

def test_resolved_scan_path_nonexistent(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    music_dir.mkdir()
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    with pytest.raises(HTTPException) as exc_info:
        api._resolved_scan_path("does_not_exist")
    assert exc_info.value.status_code == 400

def test_resolved_scan_path_absolute_escape(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    music_dir.mkdir()
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    with pytest.raises(HTTPException) as exc_info:
        api._resolved_scan_path("/etc/passwd")
    assert exc_info.value.status_code == 400

def test_resolved_scan_path_sibling_prefix(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    evil_dir = tmp_path / "music-evil"
    music_dir.mkdir()
    evil_dir.mkdir()
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    with pytest.raises(HTTPException) as exc_info:
        api._resolved_scan_path(str(evil_dir))
    assert exc_info.value.status_code == 400

def test_resolved_scan_path_absolute_subdir(monkeypatch, tmp_path):
    import config
    import api
    music_dir = tmp_path / "music"
    rock_dir = music_dir / "rock"
    rock_dir.mkdir(parents=True)
    monkeypatch.setattr(config, "MUSIC_DIR", str(music_dir))
    monkeypatch.setattr(api, "MUSIC_DIR", str(music_dir))

    resolved = api._resolved_scan_path(str(rock_dir))
    assert resolved == str(rock_dir.resolve())
