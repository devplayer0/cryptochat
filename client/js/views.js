Vue.component('NotFound', {
  template: `
    <div>
      <h1>Not Found</h1>
      <p class="lead">Couldn't find a matching visualization, sorry.</p>
      <code>¯\\_(ツ)_/¯</code>
    </div>
  `
});

Vue.component('MessagesView', {
  template: `
    <div id="wrapper">
      <nav id="sidebar">
        <ul class="list-unstyled">
          <li v-for="r in Object.keys(shared.rooms)">
            <a @click="joinRoom(r)">{{ r }}</a>
          </li>
          <li>
            <a @click="addRoom()">Add room</a>
          </li>
        </ul>
      </nav>

      <div id="content">
        <input type="text" class="form-control" placeholder="Message" v-model="message" @keyup="send">

        <ul class="list-unstyled">
          <li v-for="m in shared.messages[room]" :key="m.id">
            <h4>{{ m.sender.username }} ({{ m.sender.uuid }})</h4>
            <p>{{ m.content }}</p>
          </li>
        </ul>
      </div>
    </div>
  `,
  data() {
    return {
      room: '',
      message: '',
      shared: state,
    };
  },
  methods: {
    send: async function(e) {
      if (e.keyCode != 13) {
        return;
      }

      await fetch(`/api/rooms/${this.room}/messages`, {
        method: 'POST',
        body: JSON.stringify({
          username: this.shared.username,
          content: this.message,
        }),
      });
      this.message = '';
    },
    joinRoom: async function(name) {
      await fetch(`/api/rooms/${name}`, {
        method: 'POST',
      });
      this.room = name;
    },
    addRoom: async function() {
      const name = prompt('Name of new room');
      if (!name) {
        return
      }

      await fetch(`/api/rooms/${this.room}`, {
        method: 'POST',
      });
    },
  }
});

Vue.component('SettingsView', {
  template: `
    <div>
      <h2>Your UUID</h2>
      <p>{{ shared.uuid }}</p>
      <h2>Your fingerprint</h2>
      <p>{{ shared.fingerprint }}</p>

      <h2>Username</h2>
      <input type="text" class="form-control" placeholder="Username" v-model="shared.username">
    </div>
  `,
  data() {
    return {
      shared: state,
      message: '',
    };
  }
});
