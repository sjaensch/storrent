package torrentfile

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update .golden.json files")

func TestOpen(t *testing.T) {
	torrent, err := Open("testdata/archlinux-2019.12.01-x86_64.iso.torrent")
	require.Nil(t, err)

	goldenPath := "testdata/archlinux-2019.12.01-x86_64.iso.torrent.golden.json"
	if *update {
		serialized, err := json.MarshalIndent(torrent, "", "  ")
		require.Nil(t, err)
		ioutil.WriteFile(goldenPath, serialized, 0644)
	}

	expected := TorrentFile{}
	golden, err := ioutil.ReadFile(goldenPath)
	require.Nil(t, err)
	err = json.Unmarshal(golden, &expected)
	require.Nil(t, err)

	assert.Equal(t, expected, torrent)
}

func TestToTorrentFile(t *testing.T) {
	tests := map[string]struct {
		input  *bencodeTorrent
		output TorrentFile
		fails  bool
	}{
		"correct conversion": {
			input: &bencodeTorrent{
				Announce: "http://bttracker.debian.org:6969/announce",
				Info: bencodeInfo{
					Pieces:      "1234567890abcdefghijabcdefghij1234567890",
					PieceLength: 262144,
					Length:      351272960,
					Name:        "debian-10.2.0-amd64-netinst.iso",
				},
			},
			output: TorrentFile{
				Announce: "http://bttracker.debian.org:6969/announce",
				InfoHash: [20]byte{0x68, 0x46, 0xa2, 0xf7, 0x3a, 0x38, 0x8c, 0x3d, 0xc6, 0xb6, 0x3b, 0x56, 0x75, 0xaf, 0x1, 0x3, 0x25, 0xf0, 0x83, 0xb3},
				PieceHashes: [][20]byte{
					{49, 50, 51, 52, 53, 54, 55, 56, 57, 48, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106},
					{97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 49, 50, 51, 52, 53, 54, 55, 56, 57, 48},
				},
				PieceLength: 262144,
				Length:      351272960,
				Name:        "debian-10.2.0-amd64-netinst.iso",
				Entries:     []FileEntry{},
			},
			fails: false,
		},
		"correct conversion with multiple files": {
			input: &bencodeTorrent{
				Announce: "http://tracker.site1.com/announce",
				Info: bencodeInfo{
					Pieces:      "1234567890abcdefghijabcdefghij1234567890",
					PieceLength: 262144,
					Length:      40968192,
					Name:        "directoryName",
					Files: []bencodeFile{
						{
							Length: 111,
							Path:   []string{"subdir1", "111.txt"},
							Md5sum: "111.txtmd5sum",
						},
						{
							Length: 222,
							Path:   []string{"subdir2", "subdir3", "222.txt"},
							Md5sum: "222.txtmd5sum",
						},
					},
				},
			},
			output: TorrentFile{
				Announce: "http://tracker.site1.com/announce",
				InfoHash: [20]byte{86, 63, 132, 40, 206, 46, 13, 86, 232, 206, 75, 45, 229, 143, 164, 153, 8, 46, 68, 18},
				PieceHashes: [][20]byte{
					{49, 50, 51, 52, 53, 54, 55, 56, 57, 48, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106},
					{97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 49, 50, 51, 52, 53, 54, 55, 56, 57, 48},
				},
				PieceLength: 262144,
				Length:      40968192,
				Name:        "directoryName",
				Entries: []FileEntry{
					{
						Length: 111,
						Path:   filepath.Join("directoryName", "subdir1"),
						Name:   "111.txt",
						Md5sum: "111.txtmd5sum",
					},
					{
						Length: 222,
						Path:   filepath.Join("directoryName", "subdir2", "subdir3"),
						Name:   "222.txt",
						Md5sum: "222.txtmd5sum",
					},
				},
			},
		},
		"not enough bytes in pieces": {
			input: &bencodeTorrent{
				Announce: "http://bttracker.debian.org:6969/announce",
				Info: bencodeInfo{
					Pieces:      "1234567890abcdefghijabcdef", // Only 26 bytes
					PieceLength: 262144,
					Length:      351272960,
					Name:        "debian-10.2.0-amd64-netinst.iso",
				},
			},
			output: TorrentFile{},
			fails:  true,
		},
	}

	for _, test := range tests {
		to, err := test.input.toTorrentFile()
		if test.fails {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
		}
		assert.Equal(t, test.output, to)
	}
}
