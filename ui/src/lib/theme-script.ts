/**
 * Runs in <head> before first paint. The stored preference wins; with none
 * stored, the OS preference, as ThemeProvider reads it, so a light-OS operator
 * never sees a frame of the dark room. Dark only where nothing can be read.
 */
export const THEME_SCRIPT = `try{var t=localStorage.getItem('syndra-theme');if(t!=='light'&&t!=='dark')t=matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';document.documentElement.setAttribute('data-theme',t)}catch(e){document.documentElement.setAttribute('data-theme','dark')}`;
