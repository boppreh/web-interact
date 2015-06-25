from web import setup

class Page(object):
    def __init__(self):
        self.name = ''

    def change_name(self, new_name):
        self.name = new_name

    def say(self, message):
        template = '<p><strong>{}</strong>: {}</p>'
        line = self.html(template, self.name or 'Anon', message)
        self.set('chat', self.get('chat') + line, 'world')

setup(Page)
