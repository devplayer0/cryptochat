fetch('/api/info')
  .then(r => r.json().then(info => {
    state.uuid = info.uuid;
    state.username = info.uuid;
    state.fingerprint = info.fingerprint;
  }));

let verifyEvents = new EventSource('/api/events?stream=verification');
verifyEvents.addEventListener('message', e => {
  let v = JSON.parse(e.data);

  const method = confirm(`Is ${v.fingerprint} ${v.uuid}'s fingerprint?`) ? 'POST' : 'DELETE';
  fetch(`/api/users/${v.uuid}/verify`, {
    method,
  });
});

let messageEvents = new EventSource('/api/events?stream=messages');
messageEvents.addEventListener('message', e => {
  let m = JSON.parse(e.data);
  m.id = e.data.lastEventId;

  if (!state.messages[m.room]) {
    state.messages[m.room] = [];
  }
  console.log(m);
  state.messages[m.room].push(m);
});

setInterval(() => {
  fetch('/api/rooms').then(r => r.json().then(rooms => {
    state.rooms = rooms;
  }));
}, 3000);

const router = new VueRouter({
  mode: 'history',
  routes: [
    { path: '/', redirect: '/messages' },

    { name: 'messages', path: '/messages', component: Vue.component('MessagesView') },
    { name: 'settings', path: '/settings', component: Vue.component('SettingsView') },

    { path: '*', component: Vue.component('NotFound') }
  ]
});

const vm = new Vue({
  el: '#app',
  router
});
