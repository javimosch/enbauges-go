window.InteractiveMapHelpers = (function() {
  var CATEGORY_LABELS = {
    lieu: 'Lieu',
    service: 'Service',
    evenement: 'Événement',
    autre: 'Autre'
  };

  var CATEGORY_ICONS = {
    lieu: '📍',
    service: '🔧',
    evenement: '🎉',
    autre: '📌'
  };

  function getCategoryLabel(cat) {
    return CATEGORY_LABELS[cat] || cat;
  }

  function getCategoryIcon(cat) {
    return CATEGORY_ICONS[cat] || '📌';
  }

  function formatDate(dateStr) {
    if (!dateStr) return '';
    var d = new Date(dateStr);
    return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' });
  }

  function timeAgo(dateStr) {
    if (!dateStr) return '';
    var now = Date.now();
    var then = new Date(dateStr).getTime();
    var diffMs = now - then;
    var diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return "À l'instant";
    if (diffMin < 60) return diffMin + ' min';
    var diffH = Math.floor(diffMin / 60);
    if (diffH < 24) return diffH + ' h';
    var diffD = Math.floor(diffH / 24);
    if (diffD < 30) return diffD + ' j';
    return formatDate(dateStr);
  }

  function getCreatorId() {
    var id = localStorage.getItem('enbauges_creator_id');
    if (!id) {
      id = 'c_' + Math.random().toString(36).slice(2);
      localStorage.setItem('enbauges_creator_id', id);
    }
    return id;
  }

  function getMyConfirmations() {
    try {
      return JSON.parse(localStorage.getItem('enbauges_imap_confirmations') || '[]');
    } catch (e) {
      return [];
    }
  }

  function setMyConfirmations(arr) {
    localStorage.setItem('enbauges_imap_confirmations', JSON.stringify(arr));
  }

  function showToast(message, type) {
    var existing = document.querySelector('.imap-toast');
    if (existing) existing.remove();

    var toast = document.createElement('div');
    toast.className = 'imap-toast imap-toast--' + (type || 'success');
    toast.textContent = message;
    document.body.appendChild(toast);
    setTimeout(function() { if (toast.parentNode) toast.remove(); }, 3000);
  }

  // Default center: Le Châtelard, cœur des Bauges
  var BAUGES_CENTER = [45.685, 6.185];
  var BAUGES_ZOOM = 12;

  // Reverse geocode using Nominatim (free, rate-limited)
  var geocodeTimeout = null;
  function reverseGeocode(lat, lng, callback) {
    clearTimeout(geocodeTimeout);
    geocodeTimeout = setTimeout(function() {
      fetch('https://nominatim.openstreetmap.org/reverse?format=json&lat=' + lat + '&lon=' + lng + '&zoom=16&addressdetails=1')
        .then(function(r) { return r.json(); })
        .then(function(data) {
          callback(data.display_name || '');
        })
        .catch(function() { callback(''); });
    }, 500);
  }

  return {
    CATEGORY_LABELS: CATEGORY_LABELS,
    CATEGORY_ICONS: CATEGORY_ICONS,
    getCategoryLabel: getCategoryLabel,
    getCategoryIcon: getCategoryIcon,
    formatDate: formatDate,
    timeAgo: timeAgo,
    getCreatorId: getCreatorId,
    getMyConfirmations: getMyConfirmations,
    setMyConfirmations: setMyConfirmations,
    showToast: showToast,
    BAUGES_CENTER: BAUGES_CENTER,
    BAUGES_ZOOM: BAUGES_ZOOM,
    reverseGeocode: reverseGeocode
  };
})();
