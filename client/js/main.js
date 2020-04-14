const router = new VueRouter({
  mode: 'history',
  routes: [
    { path: '/', redirect: '/messages' },

    { name: 'messages', path: '/messages', component: Vue.component('MessagesView') },

    { path: '*', component: Vue.component('NotFound') }
  ]
});

const vm = new Vue({
  el: '#app',
  router
});
