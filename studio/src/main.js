import { createApp } from 'vue';
import { createRouter, createWebHistory } from 'vue-router';
import App from './App.vue';
import './styles/tokens.css';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/shots' },
    { path: '/shots', name: 'shots', component: () => import('./views/ShotsView.vue') },
    { path: '/episodes', name: 'episodes', component: () => import('./views/EpisodesView.vue') },
    { path: '/cast', name: 'cast', component: () => import('./views/CastView.vue') },
    { path: '/gallery', name: 'gallery', component: () => import('./views/GalleryView.vue') },
    { path: '/bible', name: 'bible', component: () => import('./views/BibleView.vue') },
    { path: '/aesthetic', name: 'aesthetic', component: () => import('./views/AestheticView.vue') },
    { path: '/cases', name: 'cases', component: () => import('./views/CaseGalleryView.vue') },
    { path: '/craft', name: 'craft', component: () => import('./views/CraftView.vue') },
    { path: '/prompt-lab', name: 'prompt-lab', component: () => import('./views/PromptLabView.vue') },
    { path: '/batch', name: 'batch', component: () => import('./views/BatchView.vue') },
    { path: '/queue', name: 'queue', component: () => import('./views/QueueView.vue') },
  ],
});

createApp(App).use(router).mount('#app');
