import socket
import threading
import re
import json

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


def setup(PageCls, host='localhost', port=8001,
          on_connected=lambda: None, on_disconnected=lambda: None):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.connect(('localhost', 8001))
    PageCls.all = {}
    reader = s.makefile('r', encoding='utf-8')
    writer = s.makefile('w', encoding='utf-8')

    def send(self, message, target=None):
        target = target or self.id
        assert '\n' not in message 
        writer.write('send {} {}\n'.format(target, message))
        writer.flush()

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

    PageCls.get = get
    PageCls.set = set
    PageCls.html = html
    PageCls.send = send
    PageCls.broadcast = broadcast

    def subroutine():
        while True:
            line = reader.readline()
            event, id, params = re.match(r'(\S+) (\S+) (.+)', line).groups()
            if event == 'connected':
                instance = PageCls()
                instance.id = id
                PageCls.all[id] = instance
            elif event == 'disconnected':
                del PageCls.all[id]
            elif event == 'call':
                call = json.loads(params)
                # Nope, not falling for that.
                if call['method'] in ['send', 'broadcast']:
                    continue
                getattr(PageCls.all[id], call['method'])(*call['params'])

    threading.Thread(target=subroutine).start()
