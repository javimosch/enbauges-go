window.IrcChatApp = (function() {
  var h = React.createElement;
  var useState = React.useState;
  var useEffect = React.useEffect;
  var useRef = React.useRef;
  var helpers = window.IrcChatHelpers;

  var API_BASE = '/chat/api';

  // ─── Nick entry screen ──────────────────────────────────────────────
  function NickScreen(props) {
    var nickState = useState(helpers.getNick() || '');
    var nick = nickState[0];
    var setNick = nickState[1];
    var errState = useState('');
    var error = errState[0];
    var setError = errState[1];

    function handleSubmit(e) {
      e.preventDefault();
      if (!helpers.isValidNick(nick)) {
        setError('Pseudo invalide (2-30 caractères, lettres/chiffres/_/-)');
        return;
      }
      helpers.setNick(nick);
      props.onNick(nick.trim());
    }

    return h('div', { className: 'irc-nick-screen' },
      h('div', { className: 'irc-nick-screen__box' },
        h('div', { className: 'irc-nick-screen__logo' }, '💬'),
        h('h1', { className: 'irc-nick-screen__title' }, 'Chat IRC'),
        h('p', { className: 'irc-nick-screen__subtitle' }, 'Massif des Bauges'),
        h('form', { onSubmit: handleSubmit },
          h('input', {
            type: 'text',
            className: 'irc-nick-screen__input',
            placeholder: 'Votre pseudo…',
            value: nick,
            onChange: function(e) { setNick(e.target.value); setError(''); },
            autoFocus: true,
            maxLength: 30
          }),
          error ? h('div', { className: 'irc-nick-screen__error' }, error) : null,
          h('button', {
            type: 'submit',
            className: 'irc-nick-screen__btn'
          }, 'Entrer →')
        ),
        h('p', { className: 'irc-nick-screen__hint' }, 'Aucune inscription nécessaire — choisissez un pseudo et discutez')
      )
    );
  }

  // ─── Emoji picker ───────────────────────────────────────────────────
  function EmojiPicker(props) {
    var emojis = helpers.getEmojiList();
    return h('div', { className: 'irc-emoji-picker' },
      emojis.map(function(em) {
        return h('button', {
          key: em,
          className: 'irc-emoji-picker__item',
          type: 'button',
          onClick: function() { props.onSelect(em); }
        }, em);
      })
    );
  }

  // ─── Channel create modal ──────────────────────────────────────────
  function CreateChannelModal(props) {
    var nameState = useState('');
    var name = nameState[0];
    var setName = nameState[1];
    var descState = useState('');
    var desc = descState[0];
    var setDesc = descState[1];
    var errState = useState('');
    var error = errState[0];
    var setError = errState[1];

    function handleSubmit(e) {
      e.preventDefault();
      if (!name.trim()) { setError('Nom du canal requis'); return; }
      if (!/^[a-zA-Z0-9_-]+$/.test(name.trim())) { setError('Nom invalide (lettres, chiffres, _ et -)'); return; }
      props.onCreate({ name: name.trim(), description: desc.trim() });
      props.onClose();
    }

    return h('div', { className: 'irc-modal-overlay', onClick: function(e) { if (e.target === e.currentTarget) props.onClose(); } },
      h('div', { className: 'irc-modal' },
        h('div', { className: 'irc-modal__header' },
          h('span', { className: 'irc-modal__title' }, '# Nouveau canal'),
          h('button', { className: 'irc-modal__close', onClick: props.onClose }, '✕')
        ),
        h('form', { onSubmit: handleSubmit, className: 'irc-modal__body' },
          h('label', { className: 'irc-field' },
            h('span', { className: 'irc-field__label' }, 'Nom'),
            h('input', {
              type: 'text',
              className: 'irc-field__input',
              placeholder: 'general, bauges-fromage, …',
              value: name,
              onChange: function(e) { setName(e.target.value); setError(''); },
              autoFocus: true,
              maxLength: 40
            })
          ),
          h('label', { className: 'irc-field' },
            h('span', { className: 'irc-field__label' }, 'Description (optionnel)'),
            h('input', {
              type: 'text',
              className: 'irc-field__input',
              placeholder: 'De quoi on cause ici ?',
              value: desc,
              onChange: function(e) { setDesc(e.target.value); },
              maxLength: 200
            })
          ),
          error ? h('div', { className: 'irc-field__error' }, error) : null,
          h('div', { className: 'irc-modal__footer' },
            h('button', { type: 'button', className: 'irc-btn irc-btn--ghost', onClick: props.onClose }, 'Annuler'),
            h('button', { type: 'submit', className: 'irc-btn irc-btn--primary' }, 'Créer #')
          )
        )
      )
    );
  }

  // ─── Main app ──────────────────────────────────────────────────────
  function App() {
    var nickState = useState('');
    var myNick = nickState[0];
    var setMyNick = nickState[1];

    var myUserId = useRef(helpers.getUserId()).current;

    var channelsState = useState([]);
    var channels = channelsState[0];
    var setChannels = channelsState[1];

    var activeChannelState = useState(null);
    var activeChannel = activeChannelState[0];
    var setActiveChannel = activeChannelState[1];

    var messagesState = useState([]);
    var messages = messagesState[0];
    var setMessages = messagesState[1];

    var inputState = useState('');
    var inputText = inputState[0];
    var setInputText = inputState[1];

    var showEmojiState = useState(false);
    var showEmoji = showEmojiState[0];
    var setShowEmoji = showEmojiState[1];

    var showCreateChannelState = useState(false);
    var showCreateChannel = showCreateChannelState[0];
    var setShowCreateChannel = showCreateChannelState[1];

    var sidebarOpenState = useState(typeof window !== 'undefined' && window.innerWidth >= 768);
    var sidebarOpen = sidebarOpenState[0];
    var setSidebarOpen = sidebarOpenState[1];

    var messagesEndRef = useRef(null);
    var messagesContainerRef = useRef(null);
    var inputRef = useRef(null);
    var pollIntervalRef = useRef(null);

    // ─── Load channels ────────────────────────────────────────────────
    function loadChannels() {
      fetch(API_BASE + '/channels')
        .then(function(r) { return r.json(); })
        .then(function(data) {
          setChannels(data.channels || []);
        })
        .catch(function(e) {
          console.error('[irc-chat] Failed to load channels:', e);
        });
    }

    // ─── Load messages ────────────────────────────────────────────────
    function loadMessages(channelId, scrollToBottom) {
      fetch(API_BASE + '/channels/' + channelId + '/messages?limit=200')
        .then(function(r) { return r.json(); })
        .then(function(data) {
          setMessages(data.messages || []);
          if (scrollToBottom !== false) {
            setTimeout(function() {
              if (messagesEndRef.current) {
                messagesEndRef.current.scrollIntoView({ behavior: 'auto' });
              }
            }, 50);
          }
        })
        .catch(function(e) {
          console.error('[irc-chat] Failed to load messages:', e);
        });
    }

    // ─── Poll for new messages ────────────────────────────────────────
    function startPolling() {
      if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
      pollIntervalRef.current = setInterval(function() {
        if (activeChannel) {
          loadMessages(activeChannel._id, false);
        }
      }, 5000);
    }

    // ─── Load channels on login ──────────────────────────────────────────
    useEffect(function() {
      if (myNick) {
        loadChannels();
      }
    }, [myNick]);

    // ─── Start polling when channel is selected ──────────────────────────
    useEffect(function() {
      if (myNick && activeChannel) {
        startPolling();
        return function() {
          if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
        };
      }
    }, [myNick, activeChannel]);

    // ─── Auto-scroll on new messages ──────────────────────────────────
    useEffect(function() {
      var container = messagesContainerRef.current;
      if (!container) return;
      // Only auto-scroll if user is near the bottom (within 150px)
      var isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 150;
      if (isNearBottom) {
        if (messagesEndRef.current) {
          messagesEndRef.current.scrollIntoView({ behavior: 'smooth' });
        }
      }
    }, [messages]);

    // ─── Send message ─────────────────────────────────────────────────
    function sendMessage(e) {
      e.preventDefault();
      if (!inputText.trim() || !activeChannel) return;

      var text = inputText.trim();
      setInputText('');
      setShowEmoji(false);

      fetch(API_BASE + '/channels/' + activeChannel._id + '/messages', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nick: myNick, text: text })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          loadMessages(activeChannel._id, true);
        })
        .catch(function(e) {
          helpers.showToast(e.message || 'Erreur', 'error');
        });
    }

    // ─── Toggle pin ────────────────────────────────────────────────────
    function togglePin(msgId) {
      fetch(API_BASE + '/messages/' + msgId + '/pin', { method: 'PATCH' })
        .then(function(r) { return r.json(); })
        .then(function() {
          if (activeChannel) loadMessages(activeChannel._id, false);
          helpers.showToast('Épinglé ✓');
        })
        .catch(function() { helpers.showToast('Erreur', 'error'); });
    }

    // ─── Toggle persist ────────────────────────────────────────────────
    function togglePersist(msgId) {
      fetch(API_BASE + '/messages/' + msgId + '/persist', { method: 'PATCH' })
        .then(function(r) { return r.json(); })
        .then(function() {
          if (activeChannel) loadMessages(activeChannel._id, false);
          helpers.showToast('Persistant ✓');
        })
        .catch(function() { helpers.showToast('Erreur', 'error'); });
    }

    // ─── Delete message ────────────────────────────────────────────────
    function deleteMessage(msgId) {
      if (!confirm('Supprimer ce message ?')) return;
      fetch(API_BASE + '/messages/' + msgId, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nick: myNick })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          if (activeChannel) loadMessages(activeChannel._id, false);
          helpers.showToast('Message supprimé');
        })
        .catch(function(e) { helpers.showToast(e.message || 'Erreur', 'error'); });
    }

    // ─── Create channel ────────────────────────────────────────────────
    function createChannel(data) {
      fetch(API_BASE + '/channels', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: data.name, description: data.description, creatorNick: myNick, creatorId: myUserId })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function(channel) {
          loadChannels();
          setActiveChannel(channel);
          helpers.showToast('Canal #' + data.name + ' créé !');
        })
        .catch(function(e) { helpers.showToast(e.message || 'Erreur', 'error'); });
    }

    // ─── Update channel ────────────────────────────────────────────────
    function updateChannel(channelId, description) {
      fetch(API_BASE + '/channels/' + channelId, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: description, creatorId: myUserId })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          loadChannels();
          helpers.showToast('Canal mis à jour ✓');
        })
        .catch(function(e) { helpers.showToast(e.message || 'Erreur', 'error'); });
    }

    // ─── Delete channel ────────────────────────────────────────────────
    function deleteChannel(channelId) {
      if (!confirm('Supprimer ce canal et tous ses messages ?')) return;
      fetch(API_BASE + '/channels/' + channelId, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ creatorId: myUserId })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          if (activeChannel && activeChannel._id === channelId) {
            setActiveChannel(null);
            setMessages([]);
          }
          loadChannels();
          helpers.showToast('Canal supprimé');
        })
        .catch(function(e) { helpers.showToast(e.message || 'Erreur', 'error'); });
    }

    // ─── Change nick ──────────────────────────────────────────────────
    function changeNick() {
      var newNick = prompt('Nouveau pseudo :', myNick);
      if (!newNick || !newNick.trim()) return;
      if (!helpers.isValidNick(newNick)) {
        helpers.showToast('Pseudo invalide (2-30 caractères)', 'error');
        return;
      }
      helpers.setNick(newNick.trim());
      setMyNick(newNick.trim());
      helpers.showToast('Pseudo changé ✓');
    }

    // ─── Select channel ────────────────────────────────────────────────
    function selectChannel(ch) {
      setActiveChannel(ch);
      setMessages([]);
      loadMessages(ch._id, true);
      // Close sidebar on mobile
      if (window.innerWidth < 768) setSidebarOpen(false);
    }

    // ─── Handle keyboard ────────────────────────────────────────────────
    useEffect(function() {
      function handleKey(e) {
        if (e.key === 'Escape') {
          if (showEmoji) { setShowEmoji(false); }
          else if (showCreateChannel) { setShowCreateChannel(false); }
        }
      }
      window.addEventListener('keydown', handleKey);
      return function() { window.removeEventListener('keydown', handleKey); };
    }, [showEmoji, showCreateChannel]);

    // ─── Not logged in yet ─────────────────────────────────────────────
    if (!myNick) {
      return h(NickScreen, {
        onNick: function(n) { setMyNick(n); }
      });
    }

    // ─── Pinned messages ────────────────────────────────────────────────
    var pinnedMessages = messages.filter(function(m) { return m.pinned; });

    // ─── Render ────────────────────────────────────────────────────────
    return h('div', { className: 'irc-app' },

      // ─── Sidebar ─────────────────────────────────────────────────────
      h('aside', { className: 'irc-sidebar' + (sidebarOpen ? ' irc-sidebar--open' : '') },
        h('div', { className: 'irc-sidebar__header' },
          h('span', { className: 'irc-sidebar__brand' }, '💬 Chat IRC'),
          h('button', {
            className: 'irc-sidebar__toggle',
            onClick: function() { setSidebarOpen(function(v) { return !v; }); },
            'aria-label': 'Fermer le panneau'
          }, '✕')
        ),

        // Nick display + change
        h('div', { className: 'irc-sidebar__nick' },
          h('span', { style: { color: helpers.nickColor(myNick) } }, myNick),
          h('button', {
            className: 'irc-sidebar__change-nick',
            onClick: changeNick,
            title: 'Changer de pseudo'
          }, '✎')
        ),

        // Create channel button
        h('button', {
          className: 'irc-sidebar__create',
          onClick: function() { setShowCreateChannel(true); }
        }, '+ Nouveau canal'),

        // Channel list
        h('div', { className: 'irc-sidebar__channels' },
          channels.length === 0
            ? h('div', { className: 'irc-sidebar__empty' }, 'Aucun canal — créez-en un !')
            : channels.map(function(ch) {
              var isActive = activeChannel && activeChannel._id === ch._id;
              var isCreator = ch.creatorId === myUserId;
              return h('div', {
                key: ch._id,
                className: 'irc-channel-item-wrapper' + (isActive ? ' irc-channel-item-wrapper--active' : '')
              },
                h('button', {
                  className: 'irc-channel-item' + (isActive ? ' irc-channel-item--active' : ''),
                  onClick: function() { selectChannel(ch); }
                },
                  h('span', { className: 'irc-channel-item__hash' }, '#'),
                  h('span', { className: 'irc-channel-item__name' }, ch.name),
                  ch.description ? h('span', { className: 'irc-channel-item__desc' }, ch.description) : null
                ),
                isCreator
                  ? h('div', { className: 'irc-channel-item__actions' },
                      h('button', {
                        className: 'irc-channel-item__action',
                        title: 'Modifier',
                        onClick: function(e) {
                          e.stopPropagation();
                          var newDesc = prompt('Description du canal :', ch.description || '');
                          if (newDesc !== null) updateChannel(ch._id, newDesc);
                        }
                      }, '✎'),
                      h('button', {
                        className: 'irc-channel-item__action irc-channel-item__action--delete',
                        title: 'Supprimer',
                        onClick: function(e) {
                          e.stopPropagation();
                          deleteChannel(ch._id);
                        }
                      }, '✕')
                    )
                  : null
              );
            })
        )
      ),

      // ─── Main content ───────────────────────────────────────────────
      h('main', { className: 'irc-main' },

        // Mobile header
        h('div', { className: 'irc-main__mobile-header' },
          h('button', {
            className: 'irc-main__menu-btn',
            onClick: function() { setSidebarOpen(true); },
            'aria-label': 'Menu'
          }, '☰'),
          activeChannel
            ? h('span', { className: 'irc-main__channel-name' }, '#' + activeChannel.name)
            : h('span', { className: 'irc-main__channel-name' }, 'Chat IRC'),
          h('span', { className: 'irc-main__my-nick', style: { color: helpers.nickColor(myNick) } }, myNick)
        ),

        activeChannel ? [

          // Pinned messages bar
          pinnedMessages.length > 0
            ? h('div', { className: 'irc-pinned-bar', key: 'pinned' },
                h('span', { className: 'irc-pinned-bar__label' }, '📌 Épinglés'),
                h('div', { className: 'irc-pinned-bar__messages' },
                  pinnedMessages.map(function(msg) {
                    return h('div', { key: msg._id, className: 'irc-pinned-msg' },
                      h('span', { className: 'irc-pinned-msg__nick', style: { color: helpers.nickColor(msg.nick) } }, '<' + msg.nick + '>'),
                      ' ',
                      h('span', { className: 'irc-pinned-msg__text' }, helpers.escapeHtml(helpers.renderEmoticons(msg.text)))
                    );
                  })
                )
              )
            : null,

          // Messages area
          h('div', { className: 'irc-messages', ref: messagesContainerRef, key: 'messages' },
            // Date separator hint
            messages.length > 0
              ? h('div', { className: 'irc-messages__separator' },
                  h('span', null, 'Début de #' + activeChannel.name + ' — les messages non épinglés/persistants expirent après 90 jours')
                )
              : null,

            messages.map(function(msg, i) {
              var isMine = msg.nick === myNick;
              var showNick = i === 0 || messages[i - 1].nick !== msg.nick;
              var prevTime = i > 0 ? messages[i - 1].createdAt : null;
              var showTime = !prevTime || (new Date(msg.createdAt) - new Date(prevTime)) > 300000; // 5 min gap

              return h('div', {
                key: msg._id,
                className: 'irc-msg' + (msg.pinned ? ' irc-msg--pinned' : '') + (msg.persistent ? ' irc-msg--persistent' : '')
              },
                showTime
                  ? h('span', { className: 'irc-msg__time' }, helpers.formatTime(msg.createdAt))
                  : null,
                showNick
                  ? h('span', { className: 'irc-msg__nick', style: { color: helpers.nickColor(msg.nick) } }, '<' + msg.nick + '>')
                  : h('span', { className: 'irc-msg__nick-spacer' }),
                h('span', { className: 'irc-msg__text' }, helpers.escapeHtml(helpers.renderEmoticons(msg.text))),
                h('span', { className: 'irc-msg__actions' },
                  h('button', {
                    className: 'irc-msg__action' + (msg.pinned ? ' irc-msg__action--active' : ''),
                    onClick: function() { togglePin(msg._id); },
                    title: msg.pinned ? 'Désépingler' : 'Épingler'
                  }, '📌'),
                  h('button', {
                    className: 'irc-msg__action' + (msg.persistent ? ' irc-msg__action--active' : ''),
                    onClick: function() { togglePersist(msg._id); },
                    title: msg.persistent ? 'Ne plus garder' : 'Garder indéfiniment'
                  }, msg.persistent ? '🔒' : '🔓'),
                  isMine
                    ? h('button', {
                        className: 'irc-msg__action irc-msg__action--delete',
                        onClick: function() { deleteMessage(msg._id); },
                        title: 'Supprimer'
                      }, '✕')
                    : null
                )
              );
            }),
            h('div', { ref: messagesEndRef })
          ),

          // Input bar
          h('div', { className: 'irc-input-bar', key: 'input' },
            showEmoji
              ? h('div', { className: 'irc-emoji-drawer' },
                  h(EmojiPicker, {
                    onSelect: function(em) {
                      setInputText(function(prev) { return prev + em; });
                      if (inputRef.current) inputRef.current.focus();
                    }
                  })
                )
              : null,
            h('form', { className: 'irc-input-bar__form', onSubmit: sendMessage },
              h('button', {
                type: 'button',
                className: 'irc-input-bar__emoji-btn',
                onClick: function() { setShowEmoji(function(v) { return !v; }); },
                'aria-label': 'Émojis'
              }, '😀'),
              h('input', {
                type: 'text',
                className: 'irc-input-bar__input',
                placeholder: 'Écrire à #' + activeChannel.name + '…',
                value: inputText,
                onChange: function(e) { setInputText(e.target.value); },
                ref: inputRef,
                autoFocus: true,
                maxLength: 1000
              }),
              h('button', {
                type: 'submit',
                className: 'irc-input-bar__send',
                disabled: !inputText.trim()
              }, '→')
            )
          )

        ] : h('div', { className: 'irc-empty' },
          h('div', { className: 'irc-empty__icon' }, '💬'),
          h('div', { className: 'irc-empty__text' }, 'Sélectionnez ou créez un canal pour commencer'),
          h('button', {
            className: 'irc-btn irc-btn--primary',
            onClick: function() { setShowCreateChannel(true); }
          }, '+ Nouveau canal')
        )
      ),

      // ─── Modals ──────────────────────────────────────────────────────
      showCreateChannel
        ? h(CreateChannelModal, {
            onCreate: createChannel,
            onClose: function() { setShowCreateChannel(false); }
          })
        : null
    );
  }

  return App;
})();

// Mount on load
document.addEventListener('DOMContentLoaded', function() {
  var root = ReactDOM.createRoot(document.getElementById('app'));
  root.render(React.createElement(window.IrcChatApp));
});
