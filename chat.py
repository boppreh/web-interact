from web import setup, PageBase, SessionBase, all_sessions

class Session(SessionBase):
    def on_open(self):
        self.name = ''
        self._update_names()

    def _update_names(self):
        template = ', '.join(['<strong>{}</strong>'] * len(all_sessions))
        users = [s.name or 'Anon' for s in all_sessions.values()]
        self.set('users_online', self.html(template, *users), 'world')

    def change_name(self, new_name):
        self.name = new_name
        self.set('name', new_name)
        self._update_names()

    def on_close(self):
        self._update_names()

class Page(PageBase):
    def on_open(self):
        self.set('name', self.session.name)

    def say(self, message):
        template = '<strong>{}</strong>: {}</br>'
        line = self.html(template, self.session.name or 'Anon', message)
        self.call('appendLine', line, target='world')

setup(Page, Session)
