window.IrcChatHelpers = (function() {

  var NICK_KEY = 'irc_chat_nick';
  var USER_ID_KEY = 'irc_chat_user_id';

  // ─── Nickname ────────────────────────────────────────────────────────
  function getNick() {
    return localStorage.getItem(NICK_KEY) || '';
  }

  function setNick(nick) {
    localStorage.setItem(NICK_KEY, nick.trim());
  }

  // Persistent user ID (UUID) — survives nick changes, used for ownership
  function getUserId() {
    var id = localStorage.getItem(USER_ID_KEY);
    if (!id) {
      id = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        var r = Math.random() * 16 | 0;
        var v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
      });
      localStorage.setItem(USER_ID_KEY, id);
    }
    return id;
  }

  function isValidNick(nick) {
    if (!nick || !nick.trim()) return false;
    var n = nick.trim();
    return n.length >= 2 && n.length <= 30 && /^[a-zA-Z0-9_àâäéèêëïîôùûüÿçÀÂÄÉÈÊËÏÎÔÙÛÜŸÇ-]+$/.test(n);
  }

  // ─── Emoji subset ────────────────────────────────────────────────────
  var EMOJI_LIST = [
    '😀','😂','🥰','😎','🤔','😅','👍','👋','❤️','🔥',
    '🎉','✨','💡','🌍','🏔️','🧀','🍷','🥖','☕','🌿',
    '🤝','💪','🙏','😊','🥳','😢','😡','🤣','👀','🎶',
    '⭐','📌','🔒','💬','📢','🏠','🚲','🌤️','🌈','🦫'
  ];

  function getEmojiList() {
    return EMOJI_LIST;
  }

  // Simple emoji detection in text — replace common emoticons
  function renderEmoticons(text) {
    if (!text) return text;
    return text
      .replace(/:-?\)/g, '😀')
      .replace(/:-?\(/g, '😢')
      .replace(/;-?\)/g, '😉')
      .replace(/:-?[Dd]/g, '😂')
      .replace(/:-?[Pp]/g, '😛')
      .replace(/:-?\//g, '🤔')
      .replace(/<3(?!\d)/g, '❤️')
      .replace(/:\'\(/g, '😭');
  }

  // ─── Time formatting ────────────────────────────────────────────────
  function timeAgo(dateStr) {
    if (!dateStr) return '';
    var now = Date.now();
    var then = new Date(dateStr).getTime();
    var diff = now - then;
    if (diff < 0) diff = 0;

    var secs = Math.floor(diff / 1000);
    var mins = Math.floor(secs / 60);
    var hours = Math.floor(mins / 60);
    var days = Math.floor(hours / 24);

    if (secs < 60) return 'à l\'instant';
    if (mins < 60) return mins + ' min';
    if (hours < 24) return hours + ' h';
    if (days < 7) return days + ' j';
    return new Date(dateStr).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' });
  }

  function formatTime(dateStr) {
    if (!dateStr) return '';
    var d = new Date(dateStr);
    return d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
  }

  function formatDate(dateStr) {
    if (!dateStr) return '';
    var d = new Date(dateStr);
    return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' });
  }

  // ─── Escape HTML ────────────────────────────────────────────────────
  function escapeHtml(text) {
    if (!text) return '';
    var div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  // ─── Toast ──────────────────────────────────────────────────────────
  function showToast(msg, type) {
    var existing = document.querySelector('.irc-toast');
    if (existing) existing.remove();

    var toast = document.createElement('div');
    toast.className = 'irc-toast irc-toast--' + (type || 'success');
    toast.textContent = msg;
    document.body.appendChild(toast);

    setTimeout(function() {
      toast.classList.add('irc-toast--out');
      setTimeout(function() { toast.remove(); }, 300);
    }, 2500);
  }

  // ─── Nick color (consistent per nick) ──────────────────────────────
  var NICK_COLORS = [
    '#7A9E7E', '#B45D3E', '#D4A03C', '#6B8FA3', '#9B7DB8',
    '#C47A5B', '#5B9E7E', '#8B6FA3', '#3D9B8F', '#C98B5B',
    '#7EB8A3', '#A37ED4', '#B8A37E', '#5BA3C4', '#D47EA3'
  ];

  function nickColor(nick) {
    if (!nick) return NICK_COLORS[0];
    var hash = 0;
    for (var i = 0; i < nick.length; i++) {
      hash = ((hash << 5) - hash) + nick.charCodeAt(i);
      hash = hash & hash;
    }
    return NICK_COLORS[Math.abs(hash) % NICK_COLORS.length];
  }

  return {
    getNick: getNick,
    setNick: setNick,
    getUserId: getUserId,
    isValidNick: isValidNick,
    getEmojiList: getEmojiList,
    renderEmoticons: renderEmoticons,
    timeAgo: timeAgo,
    formatTime: formatTime,
    formatDate: formatDate,
    escapeHtml: escapeHtml,
    showToast: showToast,
    nickColor: nickColor
  };
})();
