import socket
import threading
import re
import json
from collections import defaultdict

from urllib.parse import quote
from html import escape

class _Html(object):
    def __init__(self, template, values):
        self.value = template.format(*map(escape, values))

class _VirtualElement(object):
    def __init__(self, element_id, prepend=None, append=None):
        self.element_id = element_id
        self.prepend = prepend or []
        self.append = append or []

    def __add__(self, other):
        return _VirtualElement(self.element_id, self.prepend, self.append + [other])

    def __radd__(self, other):
        return _VirtualElement(self.element_id, [other] + self.prepend, self.append)

    @staticmethod
    def to_js(value):
        if isinstance(value, _VirtualElement):
            prepend = [_VirtualElement.to_js(v) for v in value.prepend]
            middle = 'get("{}")'.format(value.element_id)
            append = [_VirtualElement.to_js(v) for v in value.append]
            return '+'.join(prepend + [middle] + append)
        else:
            if isinstance(value, _Html):
                value = value.value
            else:
                value = escape(str(value))
            return 'decodeURIComponent("{}")'.format(quote(value))

all_pages = {}
all_sessions = {}

class _Interactive(object):
    socket_writer = None

    def send(self, message, target=None):
        target = target or self.id
        assert '\n' not in message 
        _Interactive.socket_writer.write('send {} {}\n'.format(target, message))
        _Interactive.socket_writer.flush()

    def broadcast(self, message):
        self.send(message, 'world')

    def get(self, elementId):
        return _VirtualElement(elementId)

    def set(self, elementId, value, target=None):
        value_expression = _VirtualElement.to_js(value)
        set_expression = 'set("{}", {})'.format(elementId, value_expression)
        self.send(set_expression, target=target)

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

def setup(PageCls=PageBase, SessionCls=SessionBase, host='localhost', port=8001):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.connect(('localhost', 8001))
    reader = s.makefile('r', encoding='utf-8')
    writer = s.makefile('w', encoding='utf-8')
    _Interactive.socket_writer = writer

    def subroutine():
        while True:
            line = reader.readline()
            event, id, params = re.match(r'(\S+) (\S+) (.+)', line).groups()
            if event == 'connected':
                session_id = params
                if session_id in all_sessions:
                    session = all_sessions[params]
                else:
                    session = SessionCls(session_id, {})
                    all_sessions[session_id] = session
                page = PageCls(id, session)
                session.pages[id] = page
                all_pages[id] = page
            elif event == 'disconnected':
                session = all_pages[id].session
                del session.pages[id]
                del all_pages[id]
            elif event == 'call':
                call = json.loads(params)
                method = call['method']
                # Nope, not falling for that.
                if method in ['send', 'broadcast'] or method.startswith('_'):
                    continue
                try:
                    getattr(all_pages[id], method)(*call['params'])
                except AttributeError:
                    getattr(all_pages[id].session, method)(*call['params'])


    threading.Thread(target=subroutine).start()
