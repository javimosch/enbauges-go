window.PanierLibreHelpers = (function() {

  // ─── Escape HTML ────────────────────────────────────────────────────
  function escapeHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // ─── Date formatting ────────────────────────────────────────────────
  function formatDate(dateStr) {
    if (!dateStr) return '';
    var d = new Date(dateStr);
    var day = d.getDate().toString().padStart(2, '0');
    var month = (d.getMonth() + 1).toString().padStart(2, '0');
    var year = d.getFullYear();
    return day + '/' + month + '/' + year;
  }

  function formatDateTime(dateStr) {
    if (!dateStr) return '';
    var d = new Date(dateStr);
    var day = d.getDate().toString().padStart(2, '0');
    var month = (d.getMonth() + 1).toString().padStart(2, '0');
    var year = d.getFullYear();
    var hours = d.getHours().toString().padStart(2, '0');
    var mins = d.getMinutes().toString().padStart(2, '0');
    return day + '/' + month + '/' + year + ' à ' + hours + ':' + mins;
  }

  function dateInputValue(dateStr) {
    if (!dateStr) return '';
    var d = new Date(dateStr);
    var year = d.getFullYear();
    var month = (d.getMonth() + 1).toString().padStart(2, '0');
    var day = d.getDate().toString().padStart(2, '0');
    return year + '-' + month + '-' + day;
  }

  // ─── Status helpers ──────────────────────────────────────────────────
  var STATUS_LABELS = {
    incoming: 'À venir',
    ongoing: 'En cours',
    finished: 'Terminé'
  };

  var STATUS_COLORS = {
    incoming: '#5B8DB8',
    ongoing: '#7A9E7E',
    finished: '#9CA3AF'
  };

  function statusLabel(status) {
    return STATUS_LABELS[status] || status;
  }

  function statusColor(status) {
    return STATUS_COLORS[status] || '#9CA3AF';
  }

  function computeStatus(startDate, endDate) {
    var now = new Date();
    var start = new Date(startDate);
    if (start > now) return 'incoming';
    if (endDate && new Date(endDate) < now) return 'finished';
    return 'ongoing';
  }

  // ─── Validation ──────────────────────────────────────────────────────
  function isValidEmail(email) {
    if (!email || !email.trim()) return false;
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
  }

  function isValidPhone(phone) {
    if (!phone || !phone.trim()) return false;
    return /^[\d\s+\-().]{6,}$/.test(phone.trim());
  }

  // ─── Toast ───────────────────────────────────────────────────────────
  function showToast(message, type) {
    type = type || 'success';
    var toast = document.createElement('div');
    toast.className = 'pl-toast pl-toast--' + type;
    toast.textContent = message;
    document.body.appendChild(toast);
    requestAnimationFrame(function() {
      toast.classList.add('pl-toast--visible');
    });
    setTimeout(function() {
      toast.classList.remove('pl-toast--visible');
      setTimeout(function() { toast.remove(); }, 300);
    }, 3000);
  }

  // ─── Admin session storage ──────────────────────────────────────────
  // NOTE: Admin passwords are stored in localStorage for convenience.
  // This is a trade-off: anyone with browser access can read them.
  // Acceptable for this community tool where admin passwords are simple
  // and the primary threat model is casual tampering, not targeted attack.
  var ADMIN_SESSION_KEY = 'pl_admin_sessions';

  function getAdminSessions() {
    try {
      return JSON.parse(localStorage.getItem(ADMIN_SESSION_KEY) || '{}');
    } catch (e) {
      return {};
    }
  }

  function setAdminPassword(providerId, password) {
    var sessions = getAdminSessions();
    sessions[providerId] = password;
    localStorage.setItem(ADMIN_SESSION_KEY, JSON.stringify(sessions));
  }

  function getAdminPassword(providerId) {
    var sessions = getAdminSessions();
    return sessions[providerId] || '';
  }

  function clearAdminSession(providerId) {
    var sessions = getAdminSessions();
    delete sessions[providerId];
    localStorage.setItem(ADMIN_SESSION_KEY, JSON.stringify(sessions));
  }

  // ─── Filter state persistence ────────────────────────────────────────
  var FILTER_KEY = 'pl_filter_provider';

  function saveFilterProvider(providerId) {
    localStorage.setItem(FILTER_KEY, providerId || '');
  }

  function loadFilterProvider() {
    return localStorage.getItem(FILTER_KEY) || '';
  }

  return {
    escapeHtml: escapeHtml,
    formatDate: formatDate,
    formatDateTime: formatDateTime,
    dateInputValue: dateInputValue,
    statusLabel: statusLabel,
    statusColor: statusColor,
    computeStatus: computeStatus,
    isValidEmail: isValidEmail,
    isValidPhone: isValidPhone,
    showToast: showToast,
    getAdminPassword: getAdminPassword,
    setAdminPassword: setAdminPassword,
    clearAdminSession: clearAdminSession,
    saveFilterProvider: saveFilterProvider,
    loadFilterProvider: loadFilterProvider
  };
})();
