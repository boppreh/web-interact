import socket
import threading
import re
import json
from collections import defaultdict
from base64 import b64encode

from html import escape as escape_html
from urllib.parse import quote_plus

class _Html(object):
    def __init__(self, template, values):
        if len(values):
            self.value = template.format(*map(escape_html, values))
        else:
            self.value = template

    def __repr__(self):
        return repr(self.value)

all_pages = {}
all_sessions = {}

class _Interactive(object):
    socket_writer = None

    def on_open(self):
        pass

    def on_close(self):
        pass

    def eval(self, message, target=None):
        target = target or self.id
        if '\n' in message:
            line = 'send {} eval(atob("{}"))\n'.format(target, b64encode(message.encode('utf-8')).decode('ascii'))
        else:
            line = 'send {} {}\n'.format(target, message)
        _Interactive.socket_writer.write(line)
        _Interactive.socket_writer.flush()

    def load_js(self, url):
        self.eval("""
var script = document.createElement('script');
script.src = "{}";
document.head.appendChild(script);
""".format(url))

    def call(self, method, *args, target=None):
        # Python' and Javascript's quoting rules are close enough that we can
        # use 'repr' to generate properly escaped characters.
        exp = '{}({})'.format(method, ', '.join(map(repr, args)))
        self.eval(exp, target=target)

    def set(self, element_id, value, target=None):
        if isinstance(value, _Html):
            setter = 'setRaw'
        else:
            setter = 'set'
        self.call(setter, element_id, value, target=target)

    def html(self, template, *values):
        return _Html(template, values)

class SessionBase(_Interactive):
    def __init__(self, id, pages):
        self.id = id
        self.pages = pages

class PageBase(_Interactive):
    def __init__(self, id, session):
        self.id = id
        self.session = session

def setup(PageCls=PageBase, SessionCls=SessionBase, host='localhost', port=8001, upload=None, auto_destroy_sessions=False):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.connect(('localhost', 8001))
    reader = s.makefile('r', encoding='utf-8')
    writer = s.makefile('w', encoding='utf-8')
    _Interactive.socket_writer = writer

    def subroutine():
        while True:
            line = reader.readline()
            event, id, params = re.match(r'(\S+) (\S*) (.*)', line).groups()

            if event == 'connected':
                session_id = params
                if session_id in all_sessions:
                    session = all_sessions[params]
                else:
                    session = SessionCls(session_id, {})
                    all_sessions[session_id] = session
                    session.on_open()
                page = PageCls(id, session)
                session.pages[id] = page
                all_pages[id] = page
                page.on_open()
            elif event == 'disconnected':
                if id not in all_pages:
                    continue
                session = all_pages[id].session
                del session.pages[id]
                del all_pages[id]
                page.on_close()
                if auto_destroy_sessions and len(session.pages) == 0:
                    del all_sessions[session.id]
                    session.on_close()
            elif event == 'call':
                if id not in all_pages:
                    continue
                call = json.loads(params)
                method = call['method']
                # Nope, not falling for that.
                if method in dir(_Interactive) or method.startswith('_'):
                    print('Somebody tried to call and I blocked', method)
                    continue
                try:
                    getattr(all_pages[id], method)(*call['params'])
                except AttributeError:
                    try:
                        getattr(all_pages[id].session, method)(*call['params'])
                    except AttributeError:
                        print('Somebody tried to call, but theres no', method)
            elif event == 'upload':
                path, name = id, params
                upload(path, name)


    threading.Thread(target=subroutine).start()
