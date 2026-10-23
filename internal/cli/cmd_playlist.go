package cli

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"music-manager/internal/device"
	"music-manager/internal/manifest"
)

func playlistCmd(open opener) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playlist",
		Short: "Make and change playlists",
		Long: `Playlists are ordered lists of tracks. "mm sync" writes each one as an .m3u8
file in the Playlists folder, with paths relative to it, which players and
phones list as a playlist. "mm sync --wolfgang" writes the .m3u the Wolfgang WMP01 reads instead: paths
from the card's root, with backslashes.`,
	}
	cmd.AddCommand(playlistNewCmd(open), playlistLsCmd(open), playlistShowCmd(open),
		playlistAddCmd(open), playlistRmCmd(open), playlistMoveCmd(open),
		playlistRenameCmd(open), playlistDeleteCmd(open), playlistImportCmd(open))
	return cmd
}

func playlistNewCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "new <name> [id...]",
		Short: "Make a playlist, with the given tracks in it",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			if err := a.CreatePlaylist(args[0], args[1:]...); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created %s with %s\n", args[0], tracksText(len(args)-1))
			return nil
		},
	}
}

func playlistLsCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List the playlists",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			m, err := a.Manifest()
			if err != nil {
				return err
			}
			if len(m.Playlists) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no playlists yet: mm playlist new <name> [id...]")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTRACKS\tLENGTH")
			for _, p := range m.Playlists {
				fmt.Fprintf(w, "%s\t%d\t%s\n", p.Name, len(p.Tracks), clock(playlistLength(m, p)))
			}
			return w.Flush()
		},
	}
}

func playlistShowCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show the tracks of a playlist, in order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			m, err := a.Manifest()
			if err != nil {
				return err
			}
			p, ok := findPlaylist(m, args[0])
			if !ok {
				return fmt.Errorf("no playlist named %q", args[0])
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "#\tID\tARTIST\tTITLE\tLENGTH")
			for i, id := range p.Tracks {
				t, _ := m.Get(id)
				artist, title, _ := display(t)
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i+1, id, artist, title, clock(t.PlayingLength()))
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s, %s\n", tracksText(len(p.Tracks)), clock(playlistLength(m, p)))
			return nil
		},
	}
}

func findPlaylist(m *manifest.Manifest, name string) (manifest.Playlist, bool) {
	for _, p := range m.Playlists {
		if p.Name == name {
			return p, true
		}
	}
	return manifest.Playlist{}, false
}

func playlistLength(m *manifest.Manifest, p manifest.Playlist) time.Duration {
	var total time.Duration
	for _, id := range p.Tracks {
		t, _ := m.Get(id)
		total += t.PlayingLength()
	}
	return total
}

func tracksText(n int) string {
	if n == 1 {
		return "1 track"
	}
	return fmt.Sprintf("%d tracks", n)
}

func playlistAddCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <id>...",
		Short: "Put tracks at the end of a playlist",
		Long:  "Put tracks at the end of a playlist, in the order given. A track that is already in it stays where it is.",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			n, err := a.AddToPlaylist(args[0], args[1:])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s to %s\n", tracksText(n), args[0])
			return nil
		},
	}
}

func playlistRmCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name> <id>...",
		Short: "Take tracks out of a playlist (they stay in the library)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			if err := a.RemoveFromPlaylist(args[0], args[1:]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s from %s\n", tracksText(len(args)-1), args[0])
			return nil
		},
	}
}

func playlistMoveCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "move <name> <id> <position>",
		Short: "Move a track to another place in a playlist (1 is the first)",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			pos, err := strconv.Atoi(args[2])
			if err != nil {
				return fmt.Errorf("position %q is not a number", args[2])
			}
			a, err := open()
			if err != nil {
				return err
			}
			if err := a.MoveInPlaylist(args[0], args[1], pos-1); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "moved %s to position %d in %s\n", args[1], pos, args[0])
			return nil
		},
	}
}

func playlistRenameCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <name> <new name>",
		Short: "Give a playlist another name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			if err := a.RenamePlaylist(args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "renamed %s to %s\n", args[0], args[1])
			return nil
		},
	}
}

func playlistDeleteCmd(open opener) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a playlist (its tracks stay in the library)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := open()
			if err != nil {
				return err
			}
			if err := a.DeletePlaylist(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", args[0])
			return nil
		},
	}
}

func playlistImportCmd(open opener) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import <USERPLn.PL> <name>",
		Short: "Make a playlist from one of the Wolfgang WMP01's own playlist files",
		Long: `Read a playlist file from the Wolfgang WMP01 (USERPL1.PL to USERPL3.PL, in the
player's root folder; other players do not have them) and make a playlist of
its songs. The player keeps only the old file names, cut to 26 characters, so
each is matched to a track by its title; a leading track number tells apart
songs of the same title.

Every entry is listed. A "~" marks one matched by the start of the title only,
a "?" one that fits several tracks (not added; the candidates are listed) and a
"-" one that was not found. Check them, and use "mm playlist add" for what is
missing. --dry-run only lists.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			names, err := device.ReadWolfgangPlaylist(data)
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			a, err := open()
			if err != nil {
				return err
			}
			library, err := a.Manifest()
			if err != nil {
				return err
			}
			matches := device.MatchWolfgangNames(names, library.Tracks)

			out := cmd.OutOrStdout()
			var ids []string
			problems := 0
			for _, m := range matches {
				switch {
				case m.Track != nil:
					artist, title, _ := display(*m.Track)
					mark := " "
					if m.Partial {
						mark, problems = "~", problems+1
					}
					fmt.Fprintf(out, "%s %-28s %s  %s - %s\n", mark, m.Name, m.Track.ID, artist, title)
					ids = append(ids, m.Track.ID)
				case len(m.Candidates) > 0:
					problems++
					fmt.Fprintf(out, "? %-28s fits %d tracks:\n", m.Name, len(m.Candidates))
					for _, c := range m.Candidates {
						artist, title, album := display(c)
						fmt.Fprintf(out, "      %s  %s - %s (%s)\n", c.ID, artist, title, album)
					}
				default:
					problems++
					fmt.Fprintf(out, "- %-28s not found\n", m.Name)
				}
			}
			if dryRun {
				fmt.Fprintf(out, "%s found of %d; nothing was made\n", tracksText(len(ids)), len(names))
				return nil
			}
			if err := a.CreatePlaylist(args[1], ids...); err != nil {
				return err
			}
			fmt.Fprintf(out, "created %s with %s", args[1], tracksText(len(ids)))
			if problems > 0 {
				fmt.Fprintf(out, "; %d of the %d entries need a look (~ ? -)", problems, len(names))
			}
			fmt.Fprintln(out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only list what each entry matches")
	return cmd
}
