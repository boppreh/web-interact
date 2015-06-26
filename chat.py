from web import setup, PageBase, SessionBase

class Session(SessionBase):
    def __init__(self, id, pages):
        SessionBase.__init__(self, id, pages)
        self.name = ''

    def change_name(self, new_name):
        self.name = new_name
        # self.set('name', self.name)

class Page(PageBase):
    def __init__(self, id, session):
        PageBase.__init__(self, id, session)
        self.set('name', session.name)

    def say(self, message):
        template = '<p><strong>{}</strong>: {}</p>'
        line = self.html(template, self.session.name or 'Anon', message)
        self.set('chat', self.get('chat') + line, 'world')

setup(Page, Session)
