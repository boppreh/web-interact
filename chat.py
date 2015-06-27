from web import setup, PageBase, SessionBase, all_pages

class Session(SessionBase):
    def __init__(self, id, pages):
        SessionBase.__init__(self, id, pages)
        self.name = ''

class Page(PageBase):
    def __init__(self, id, session):
        PageBase.__init__(self, id, session)
        self.set('name', session.name)
        self._update_names()

    def _update_names(self):
        names = [s.session.name or 'Anon' for s in all_pages.values()]
        self.set('users_online', ', '.join(names), 'world')

    def change_name(self, new_name):
        self.session.name = new_name
        self.set('name', new_name)
        self._update_names()

    def say(self, message):
        template = '<p><strong>{}</strong>: {}</p>'
        line = self.html(template, self.session.name or 'Anon', message)
        self.set('chat', self.get('chat') + line, 'world')

    def __del__(self):
        self._update_names()

setup(Page, Session)
