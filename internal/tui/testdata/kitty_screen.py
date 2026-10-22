# Feeds a file of terminal output to kitty's own terminal core, without a
# window, and prints what kitty made of it as JSON. Run with:
#
#   kitty +runpy "import sys; sys.argv=['x', 'FILE']; exec(open('this file').read())"
import json
import sys

from kitty.fast_data_types import Screen


class Callbacks:
    def __init__(self):
        self.replies = b''

    def write(self, data):
        self.replies += bytes(data)

    def __getattr__(self, name):
        return lambda *a, **k: None


def feed(screen, data):
    data = memoryview(data)
    while data:
        dest = screen.test_create_write_buffer()
        n = screen.test_commit_write_buffer(data, dest)
        data = data[n:]
        screen.test_parse_written_data()


cb = Callbacks()
cols, rows = (int(sys.argv[2]), int(sys.argv[3])) if len(sys.argv) > 3 else (80, 24)
screen = Screen(cb, rows, cols, 0, 10, 20, 0, cb)  # cells of 10x20 pixels
data = open(sys.argv[1], 'rb').read()
data = data.replace(b'q=2', b'q=0')  # ask kitty to answer, so that errors show
feed(screen, data)
screen.update_only_line_graphics_data()
layers = screen.grman.update_layers(0, -1.0, 1.0, 0.1, 0.2, screen.columns, screen.lines, 10, 20)
print(json.dumps({
    'images': screen.grman.image_count,
    'replies': cb.replies.decode('latin-1'),
    'layers': [l['src_rect'] for l in layers],
}))
