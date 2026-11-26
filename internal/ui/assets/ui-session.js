(function (scope) {
  function initializeUISession(location, history, pageToken) {
    const fragmentToken = new URLSearchParams(location.hash.slice(1)).get('ui_session') || '';
    let token = fragmentToken || pageToken || '';
    if (fragmentToken) {
      try { history.replaceState(null, '', location.pathname + location.search); } catch {}
    }
    return {
      get token() { return token; },
      clear() {
        token = '';
      },
    };
  }

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { initializeUISession };
  }
  if (scope) scope.initializeNexoraUISession = initializeUISession;
})(typeof window === 'undefined' ? globalThis : window);
