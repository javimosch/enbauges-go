window.PanierLibreApp = (function() {
  var h = React.createElement;
  var useState = React.useState;
  var useEffect = React.useEffect;
  var useRef = React.useRef;
  var helpers = window.PanierLibreHelpers;

  var API_BASE = '/panier-libre/api';

  // ═══════════════════════════════════════════════════════════════════
  // Modal wrapper
  // ═══════════════════════════════════════════════════════════════════
  function Modal(props) {
    if (!props.open) return null;
    return h('div', { className: 'pl-modal-overlay', onClick: function(e) { if (e.target === e.currentTarget) props.onClose(); } },
      h('div', { className: 'pl-modal' },
        h('div', { className: 'pl-modal__header' },
          h('h3', { className: 'pl-modal__title' }, props.title),
          h('button', { className: 'pl-modal__close', onClick: props.onClose }, '✕')
        ),
        h('div', { className: 'pl-modal__body' }, props.children)
      )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Status badge
  // ═══════════════════════════════════════════════════════════════════
  function StatusBadge(props) {
    return h('span', {
      className: 'pl-status pl-status--' + props.status
    }, helpers.statusLabel(props.status));
  }

  // ═══════════════════════════════════════════════════════════════════
  // Create/Edit Provider Modal
  // ═══════════════════════════════════════════════════════════════════
  function ProviderModal(props) {
    var formRef = useRef({
      name: props.provider ? props.provider.name : '',
      description: props.provider ? props.provider.description : '',
      email: props.provider ? (props.provider.email || '') : '',
      phone: props.provider ? (props.provider.phone || '') : '',
      address: props.provider ? (props.provider.address || '') : '',
      lat: props.provider && props.provider.location ? props.provider.location.lat : '',
      lng: props.provider && props.provider.location ? props.provider.location.lng : '',
      adminPassword: '',
    });

    var errState = useState('');
    var error = errState[0];
    var setError = errState[1];

    function handleSubmit(e) {
      e.preventDefault();
      var f = formRef.current;
      if (!f.name.trim()) { setError('Le nom est requis'); return; }
      if (!props.provider && (!f.adminPassword || f.adminPassword.length < 4)) {
        setError('Mot de passe admin requis (4 caractères min)');
        return;
      }

      var data = {
        name: f.name.trim(),
        description: f.description.trim(),
        email: f.email.trim(),
        phone: f.phone.trim(),
        address: f.address.trim(),
        adminPassword: f.adminPassword || undefined
      };

      if (f.lat !== '' && f.lng !== '') {
        data.location = { lat: Number(f.lat), lng: Number(f.lng) };
      }

      props.onSubmit(data);
    }

    var f = formRef.current;
    return h(Modal, { open: props.open, onClose: props.onClose, title: props.provider ? 'Modifier le fournisseur' : 'Nouveau fournisseur' },
      h('form', { onSubmit: handleSubmit, className: 'pl-form' },
        h('label', { className: 'pl-label' }, 'Nom *',
          h('input', { className: 'pl-input', defaultValue: f.name, maxLength: 80, onChange: function(e) { f.name = e.target.value; }, placeholder: 'Ex: AMAP des Bauges' })
        ),
        h('label', { className: 'pl-label' }, 'Description',
          h('textarea', { className: 'pl-input pl-input--textarea', defaultValue: f.description, maxLength: 500, onChange: function(e) { f.description = e.target.value; }, placeholder: 'Décrivez votre activité…', rows: 3 })
        ),
        h('div', { className: 'pl-form-row' },
          h('label', { className: 'pl-label pl-label--flex' }, 'Email',
            h('input', { className: 'pl-input', type: 'email', defaultValue: f.email, maxLength: 120, onChange: function(e) { f.email = e.target.value; }, placeholder: 'contact@exemple.fr' })
          ),
          h('label', { className: 'pl-label pl-label--flex' }, 'Téléphone',
            h('input', { className: 'pl-input', type: 'tel', defaultValue: f.phone, maxLength: 30, onChange: function(e) { f.phone = e.target.value; }, placeholder: '06 00 00 00 00' })
          )
        ),
        h('label', { className: 'pl-label' }, 'Adresse',
          h('input', { className: 'pl-input', defaultValue: f.address, maxLength: 300, onChange: function(e) { f.address = e.target.value; }, placeholder: '1 Rue du Village, 73000 Chambéry' })
        ),
        h('div', { className: 'pl-form-row' },
          h('label', { className: 'pl-label pl-label--flex' }, 'Latitude',
            h('input', { className: 'pl-input', type: 'number', step: '0.0001', defaultValue: f.lat, onChange: function(e) { f.lat = e.target.value; }, placeholder: '45.57' })
          ),
          h('label', { className: 'pl-label pl-label--flex' }, 'Longitude',
            h('input', { className: 'pl-input', type: 'number', step: '0.0001', defaultValue: f.lng, onChange: function(e) { f.lng = e.target.value; }, placeholder: '6.12' })
          )
        ),
        !props.provider ? h('label', { className: 'pl-label' }, 'Mot de passe admin *',
          h('input', { className: 'pl-input', type: 'password', defaultValue: '', minLength: 4, onChange: function(e) { f.adminPassword = e.target.value; }, placeholder: 'Minimum 4 caractères' })
        ) : null,
        error ? h('div', { className: 'pl-error' }, error) : null,
        h('button', { type: 'submit', className: 'pl-btn pl-btn--primary' }, props.provider ? 'Enregistrer' : 'Créer le fournisseur')
      )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Create/Edit Basket Modal
  // ═══════════════════════════════════════════════════════════════════
  function BasketModal(props) {
    var existing = props.basket;
    var initItems = existing ? existing.items.map(function(it) { return { key: it.key, quantity: it.quantity }; }) : [{ key: '', quantity: 0 }];

    var titleState = useState(existing ? existing.title : '');
    var descState = useState(existing ? existing.description : '');
    var startState = useState(existing ? helpers.dateInputValue(existing.startDate) : '');
    var endState = useState(existing && existing.endDate ? helpers.dateInputValue(existing.endDate) : '');
    var itemsState = useState(initItems);
    var errState = useState('');
    var error = errState[0]; var setError = errState[1];

    var title = titleState[0]; var setTitle = titleState[1];
    var desc = descState[0]; var setDesc = descState[1];
    var start = startState[0]; var setStart = startState[1];
    var end = endState[0]; var setEnd = endState[1];
    var items = itemsState[0]; var setItems = itemsState[1];

    function addItem() {
      setItems(items.concat([{ key: '', quantity: 0 }]));
    }

    function removeItem(idx) {
      setItems(items.filter(function(_, i) { return i !== idx; }));
    }

    function updateItem(idx, field, value) {
      var newItems = items.slice();
      newItems[idx] = Object.assign({}, newItems[idx]);
      newItems[idx][field] = field === 'quantity' ? Math.max(0, Number(value) || 0) : value;
      setItems(newItems);
    }

    function handleSubmit(e) {
      e.preventDefault();
      if (!title.trim()) { setError('Le titre est requis'); return; }
      if (!start) { setError('Date de début requise'); return; }
      var cleanItems = items.filter(function(it) { return it.key.trim() && it.quantity > 0; });
      if (cleanItems.length === 0) { setError('Au moins un article avec une quantité > 0'); return; }

      var data = {
        providerId: props.providerId,
        title: title.trim(),
        description: desc.trim(),
        items: cleanItems,
        startDate: start,
        endDate: end || null,
        adminPassword: props.adminPassword
      };

      props.onSubmit(data);
    }

    return h(Modal, { open: props.open, onClose: props.onClose, title: existing ? 'Modifier le panier' : 'Nouveau panier' },
      h('form', { onSubmit: handleSubmit, className: 'pl-form' },
        h('label', { className: 'pl-label' }, 'Titre *',
          h('input', { className: 'pl-input', value: title, maxLength: 100, onChange: function(e) { setTitle(e.target.value); setError(''); }, placeholder: 'Panier légumes d\'été' })
        ),
        h('label', { className: 'pl-label' }, 'Description',
          h('textarea', { className: 'pl-input pl-input--textarea', value: desc, maxLength: 1000, onChange: function(e) { setDesc(e.target.value); }, placeholder: 'Contenu, conditions…', rows: 3 })
        ),
        h('div', { className: 'pl-form-row' },
          h('label', { className: 'pl-label pl-label--flex' }, 'Début *',
            h('input', { className: 'pl-input', type: 'date', value: start, onChange: function(e) { setStart(e.target.value); setError(''); } })
          ),
          h('label', { className: 'pl-label pl-label--flex' }, 'Fin',
            h('input', { className: 'pl-input', type: 'date', value: end, onChange: function(e) { setEnd(e.target.value); } })
          )
        ),
        h('div', { className: 'pl-label' }, 'Articles *',
          items.map(function(item, idx) {
            return h('div', { key: idx, className: 'pl-item-row' },
              h('input', { className: 'pl-input pl-input--item-key', value: item.key, maxLength: 60, placeholder: 'Article', onChange: function(e) { updateItem(idx, 'key', e.target.value); } }),
              h('input', { className: 'pl-input pl-input--item-qty', type: 'number', min: '0', value: item.quantity || '', placeholder: 'Qté', onChange: function(e) { updateItem(idx, 'quantity', e.target.value); } }),
              h('button', { type: 'button', className: 'pl-item-remove', onClick: function() { removeItem(idx); }, disabled: items.length <= 1 }, '✕')
            );
          }),
          h('button', { type: 'button', className: 'pl-btn pl-btn--small', onClick: addItem }, '+ Article')
        ),
        error ? h('div', { className: 'pl-error' }, error) : null,
        h('button', { type: 'submit', className: 'pl-btn pl-btn--primary' }, existing ? 'Enregistrer' : 'Créer le panier')
      )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Booking Modal (user creates a booking)
  // ═══════════════════════════════════════════════════════════════════
  function BookingModal(props) {
    var basket = props.basket;
    if (!basket) return null;

    var nameState = useState('');
    var emailState = useState('');
    var phoneState = useState('');
    var notesState = useState('');
    var selectionsState = useState(
      basket.items.map(function(it) { return { key: it.key, quantity: 0, available: it.available }; })
    );
    var errState = useState('');

    function updateSelection(idx, qty) {
      var sel = selectionsState[0].slice();
      var maxAvail = sel[idx].available;
      sel[idx] = Object.assign({}, sel[idx], { quantity: Math.min(Math.max(0, Number(qty) || 0), maxAvail) });
      selectionsState[1](sel);
    }

    function handleSubmit(e) {
      e.preventDefault();
      var name = nameState[0].trim();
      var email = emailState[0].trim();
      var phone = phoneState[0].trim();
      var notes = notesState[0].trim();
      var sel = selectionsState[0];

      if (!name) { errState[1]('Le nom est requis'); return; }
      if (!email && !phone) { errState[1]('Email ou téléphone requis'); return; }
      var selected = sel.filter(function(s) { return s.quantity > 0; });
      if (selected.length === 0) { errState[1]('Sélectionnez au moins un article'); return; }

      props.onSubmit({
        name: name,
        email: email,
        phone: phone,
        extraNotes: notes,
        items: selected.map(function(s) { return { key: s.key, quantity: s.quantity }; })
      });
    }

    var sel = selectionsState[0];
    return h(Modal, { open: props.open, onClose: props.onClose, title: 'Réserver — ' + basket.title },
      h('form', { onSubmit: handleSubmit, className: 'pl-form' },
        h('label', { className: 'pl-label' }, 'Nom *',
          h('input', { className: 'pl-input', value: nameState[0], maxLength: 80, onChange: function(e) { nameState[1](e.target.value); errState[1](''); }, placeholder: 'Votre nom' })
        ),
        h('div', { className: 'pl-form-row' },
          h('label', { className: 'pl-label pl-label--flex' }, 'Email',
            h('input', { className: 'pl-input', type: 'email', value: emailState[0], maxLength: 120, onChange: function(e) { emailState[1](e.target.value); }, placeholder: 'vous@exemple.fr' })
          ),
          h('label', { className: 'pl-label pl-label--flex' }, 'Téléphone',
            h('input', { className: 'pl-input', type: 'tel', value: phoneState[0], maxLength: 30, onChange: function(e) { phoneState[1](e.target.value); }, placeholder: '06 00 00 00 00' })
          )
        ),
        h('label', { className: 'pl-label' }, 'Notes',
          h('textarea', { className: 'pl-input pl-input--textarea', value: notesState[0], maxLength: 500, onChange: function(e) { notesState[1](e.target.value); }, placeholder: 'Précisions, allergies…', rows: 2 })
        ),
        h('div', { className: 'pl-label' }, 'Sélection des articles',
          sel.map(function(s, idx) {
            return h('div', { key: s.key, className: 'pl-booking-item' },
              h('span', { className: 'pl-booking-item__key' }, helpers.escapeHtml(s.key)),
              h('span', { className: 'pl-booking-item__avail' }, s.available + ' dispo'),
              h('input', {
                className: 'pl-input pl-input--item-qty',
                type: 'number',
                min: '0',
                max: String(s.available),
                value: s.quantity || '',
                placeholder: '0',
                onChange: function(e) { updateSelection(idx, e.target.value); }
              })
            );
          })
        ),
        errState[0] ? h('div', { className: 'pl-error' }, errState[0]) : null,
        h('button', { type: 'submit', className: 'pl-btn pl-btn--primary' }, 'Confirmer la réservation')
      )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Edit Booking Modal (used by both users and admins)
  // ═══════════════════════════════════════════════════════════════════
  function EditBookingModal(props) {
    var booking = props.booking;
    var basket = props.basket;
    var isAdmin = props.isAdmin;
    var adminPassword = props.adminPassword;
    if (!booking || !basket) return null;

    var nameState = useState(booking.name || '');
    var emailState = useState(booking.email || '');
    var phoneState = useState(booking.phone || '');
    var notesState = useState(booking.extraNotes || '');
    var errState = useState('');

    // Build selections from existing booking items + basket available info
    var initSel = basket.items.map(function(it) {
      var booked = (booking.items || []).find(function(bi) { return bi.key === it.key; });
      return {
        key: it.key,
        quantity: booked ? booked.quantity : 0,
        available: it.available + (booked ? booked.quantity : 0) // add back this booking's own quantity
      };
    });
    var selectionsState = useState(initSel);

    function updateSelection(idx, qty) {
      var sel = selectionsState[0].slice();
      var maxAvail = sel[idx].available;
      sel[idx] = Object.assign({}, sel[idx], { quantity: Math.min(Math.max(0, Number(qty) || 0), maxAvail) });
      selectionsState[1](sel);
    }

    function handleSubmit(e) {
      e.preventDefault();
      var name = nameState[0].trim();
      var email = emailState[0].trim();
      var phone = phoneState[0].trim();
      var notes = notesState[0].trim();
      var sel = selectionsState[0];

      if (!name) { errState[1]('Le nom est requis'); return; }
      if (!isAdmin && !emailState[0].trim() && !phoneState[0].trim()) { errState[1]('Email ou téléphone requis'); return; }
      var selected = sel.filter(function(s) { return s.quantity > 0; });
      if (selected.length === 0) { errState[1]('Sélectionnez au moins un article'); return; }

      var data = {
        name: name,
        email: email,
        phone: phone,
        extraNotes: notes,
        items: selected.map(function(s) { return { key: s.key, quantity: s.quantity }; })
      };

      // Auth: admin sends password, user sends email/phone
      if (isAdmin && adminPassword) {
        data.adminPassword = adminPassword;
      } else {
        if (email) data.email = email;
        if (phone) data.phone = phone;
      }

      props.onSubmit(data);
    }

    var sel = selectionsState[0];
    return h(Modal, { open: props.open, onClose: props.onClose, title: 'Modifier la réservation' },
      h('form', { onSubmit: handleSubmit, className: 'pl-form' },
        h('label', { className: 'pl-label' }, 'Nom *',
          h('input', { className: 'pl-input', value: nameState[0], maxLength: 80, onChange: function(e) { nameState[1](e.target.value); errState[1](''); }, placeholder: 'Votre nom' })
        ),
        !isAdmin ? h('div', { className: 'pl-form-row' },
          h('label', { className: 'pl-label pl-label--flex' }, 'Email',
            h('input', { className: 'pl-input', type: 'email', value: emailState[0], maxLength: 120, onChange: function(e) { emailState[1](e.target.value); }, placeholder: 'vous@exemple.fr' })
          ),
          h('label', { className: 'pl-label pl-label--flex' }, 'Téléphone',
            h('input', { className: 'pl-input', type: 'tel', value: phoneState[0], maxLength: 30, onChange: function(e) { phoneState[1](e.target.value); }, placeholder: '06 00 00 00 00' })
          )
        ) : null,
        h('label', { className: 'pl-label' }, 'Notes',
          h('textarea', { className: 'pl-input pl-input--textarea', value: notesState[0], maxLength: 500, onChange: function(e) { notesState[1](e.target.value); }, placeholder: 'Précisions…', rows: 2 })
        ),
        h('div', { className: 'pl-label' }, 'Sélection des articles',
          sel.map(function(s, idx) {
            return h('div', { key: s.key, className: 'pl-booking-item' },
              h('span', { className: 'pl-booking-item__key' }, helpers.escapeHtml(s.key)),
              h('span', { className: 'pl-booking-item__avail' }, s.available + ' dispo'),
              h('input', {
                className: 'pl-input pl-input--item-qty',
                type: 'number',
                min: '0',
                max: String(s.available),
                value: s.quantity || '',
                placeholder: '0',
                onChange: function(e) { updateSelection(idx, e.target.value); }
              })
            );
          })
        ),
        errState[0] ? h('div', { className: 'pl-error' }, errState[0]) : null,
        h('button', { type: 'submit', className: 'pl-btn pl-btn--primary' }, 'Enregistrer les modifications')
      )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Booking Lookup Modal
  // ═══════════════════════════════════════════════════════════════════
  function LookupModal(props) {
    var emailState = useState('');
    var phoneState = useState('');
    var resultsState = useState(null);
    var loadingState = useState(false);

    var email = emailState[0]; var setEmail = emailState[1];
    var phone = phoneState[0]; var setPhone = phoneState[1];

    function doLookup() {
      loadingState[1](true);
      fetch(API_BASE + '/bookings/lookup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: email.trim(), phone: phone.trim() })
      })
        .then(function(r) { return r.json(); })
        .then(function(data) {
          resultsState[1](data.bookings || []);
          loadingState[1](false);
        })
        .catch(function() {
          loadingState[1](false);
          helpers.showToast('Erreur de recherche', 'error');
        });
    }

    function handleLookup(e) {
      e.preventDefault();
      doLookup();
    }

    // Edit booking state
    var editingBookingState = useState(null);
    var editingBooking = editingBookingState[0]; var setEditingBooking = editingBookingState[1];
    var editBasketState = useState(null);
    var editBasket = editBasketState[0]; var setEditBasket = editBasketState[1];
    var editLoadingState = useState(false);

    function startEditBooking(bk) {
      editLoadingState[1](true);
      // Fetch basket details for available items
      fetch(API_BASE + '/baskets/' + bk.basketId)
        .then(function(r) { return r.json(); })
        .then(function(basket) {
          setEditBasket(basket);
          setEditingBooking(bk);
          editLoadingState[1](false);
        })
        .catch(function() {
          helpers.showToast('Impossible de charger les détails du panier', 'error');
          editLoadingState[1](false);
        });
    }

    function submitEditBooking(data) {
      // Pass original lookup email/phone as ownerEmail/ownerPhone for auth
      // so user can change their email/phone in the form without breaking auth
      var authData = Object.assign({}, data, {
        ownerEmail: email.trim(),
        ownerPhone: phone.trim()
      });
      fetch(API_BASE + '/bookings/' + editingBooking._id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(authData)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function(updated) {
          helpers.showToast('Réservation modifiée ✓');
          setEditingBooking(null);
          setEditBasket(null);
          // Refresh lookup results
          doLookup();
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    function cancelBooking(bookingId) {
      if (!confirm('Annuler cette réservation ?')) return;
      var data = {};
      if (email.trim()) data.email = email.trim();
      if (phone.trim()) data.phone = phone.trim();
      fetch(API_BASE + '/bookings/' + bookingId, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Réservation annulée ✓');
          // Refresh results
          resultsState[1]((resultsState[0] || []).filter(function(bk) { return bk._id !== bookingId; }));
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    var results = resultsState[0];
    return h('div', null,
      // Edit booking modal (outside LookupModal so it doesn't nest)
      h(EditBookingModal, {
        key: editingBooking ? editingBooking._id : 'no-edit',
        open: !!editingBooking,
        booking: editingBooking,
        basket: editBasket,
        isAdmin: false,
        onClose: function() { setEditingBooking(null); setEditBasket(null); },
        onSubmit: submitEditBooking
      }),
    h(Modal, { open: props.open, onClose: props.onClose, title: 'Mes réservations' },
      h('form', { onSubmit: handleLookup, className: 'pl-form' },
        h('p', { className: 'pl-hint' }, 'Entrez votre email ou téléphone pour retrouver vos réservations.'),
        h('div', { className: 'pl-form-row' },
          h('label', { className: 'pl-label pl-label--flex' }, 'Email',
            h('input', { className: 'pl-input', type: 'email', value: email, onChange: function(e) { setEmail(e.target.value); }, placeholder: 'vous@exemple.fr' })
          ),
          h('label', { className: 'pl-label pl-label--flex' }, 'Téléphone',
            h('input', { className: 'pl-input', type: 'tel', value: phone, onChange: function(e) { setPhone(e.target.value); }, placeholder: '06 00 00 00 00' })
          )
        ),
        h('button', { type: 'submit', className: 'pl-btn pl-btn--primary', disabled: loadingState[0] }, loadingState[0] ? 'Recherche…' : 'Rechercher')
      ),
      results !== null ? h('div', { className: 'pl-lookup-results' },
        results.length === 0
          ? h('p', { className: 'pl-hint' }, 'Aucune réservation trouvée.')
          : results.map(function(bk) {
              var canCancel = bk.basketStatus === 'ongoing' || bk.basketStatus === 'incoming';
              return h('div', { key: bk._id, className: 'pl-lookup-card' },
                h('div', { className: 'pl-lookup-card__header' },
                  h('span', { className: 'pl-lookup-card__provider' }, helpers.escapeHtml(bk.providerName)),
                  h('span', { className: 'pl-lookup-card__basket' }, helpers.escapeHtml(bk.basketTitle)),
                  h(StatusBadge, { status: bk.basketStatus })
                ),
                h('div', { className: 'pl-lookup-card__items' },
                  bk.items.map(function(it) {
                    return h('span', { key: it.key, className: 'pl-tag' }, helpers.escapeHtml(it.key) + ' ×' + it.quantity);
                  })
                ),
                h('div', { className: 'pl-lookup-card__meta' },
                  h('span', null, helpers.escapeHtml(bk.name)),
                  h('span', null, helpers.formatDateTime(bk.createdAt)),
                  bk.extraNotes ? h('span', null, helpers.escapeHtml(bk.extraNotes)) : null
                ),
                canCancel ? h('div', { className: 'pl-lookup-card__actions' },
                  h('button', {
                    className: 'pl-btn pl-btn--small',
                    onClick: function() { startEditBooking(bk); },
                    disabled: editLoadingState[0]
                  }, '✎ Modifier'),
                  h('button', {
                    className: 'pl-btn pl-btn--danger pl-btn--small',
                    onClick: function() { cancelBooking(bk._id); }
                  }, 'Annuler')
                ) : null
              );
            })
      ) : null
    )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Admin Login Modal (password verification)
  // ═══════════════════════════════════════════════════════════════════
  function AdminLoginModal(props) {
    var pwState = useState('');
    var errState = useState('');

    function handleSubmit(e) {
      e.preventDefault();
      var pw = pwState[0];
      if (!pw || pw.length < 4) { errState[1]('Mot de passe requis'); return; }

      fetch(API_BASE + '/providers/' + props.providerId + '/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ adminPassword: pw })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.setAdminPassword(props.providerId, pw);
          props.onVerified(pw);
        })
        .catch(function(e) { errState[1](e.message || 'Erreur'); });
    }

    return h(Modal, { open: props.open, onClose: props.onClose, title: 'Accès admin' },
      h('form', { onSubmit: handleSubmit, className: 'pl-form' },
        h('p', { className: 'pl-hint' }, 'Entrez le mot de passe admin pour gérer ce fournisseur.'),
        h('label', { className: 'pl-label' }, 'Mot de passe',
          h('input', { className: 'pl-input', type: 'password', value: pwState[0], onChange: function(e) { pwState[1](e.target.value); errState[1](''); }, autoFocus: true, minLength: 4 })
        ),
        errState[0] ? h('div', { className: 'pl-error' }, errState[0]) : null,
        h('button', { type: 'submit', className: 'pl-btn pl-btn--primary' }, 'Vérifier')
      )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Admin Bookings View
  // ═══════════════════════════════════════════════════════════════════
  function AdminBookingsView(props) {
    var basket = props.basket;
    var adminPw = props.adminPassword;
    var bookingsState = useState([]);
    var bookings = bookingsState[0]; var setBookings = bookingsState[1];

    // Edit booking state
    var editingBookingState = useState(null);
    var editingBooking = editingBookingState[0]; var setEditingBooking = editingBookingState[1];

    function loadBookings() {
      if (!basket || !adminPw) return;
      fetch(API_BASE + '/baskets/' + basket._id + '/bookings/list', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ adminPassword: adminPw })
      })
        .then(function(r) { return r.json(); })
        .then(function(data) { setBookings(data.bookings || []); })
        .catch(function() { helpers.showToast('Erreur de chargement', 'error'); });
    }

    useEffect(function() { loadBookings(); }, [basket && basket._id, adminPw]);

    function submitAdminEditBooking(data) {
      fetch(API_BASE + '/bookings/' + editingBooking._id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(Object.assign({}, data, { adminPassword: adminPw }))
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Réservation modifiée ✓');
          setEditingBooking(null);
          loadBookings();
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    function deleteBooking(bookingId) {
      if (!confirm('Annuler cette réservation ?')) return;
      fetch(API_BASE + '/bookings/' + bookingId, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ adminPassword: adminPw })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          setBookings(bookings.filter(function(b) { return b._id !== bookingId; }));
          helpers.showToast('Réservation annulée ✓');
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    return h('div', { className: 'pl-admin-bookings' },
      h('h4', { className: 'pl-admin-bookings__title' }, 'Réservations (' + bookings.length + ')'),
      // Admin edit booking modal
      h(EditBookingModal, {
        key: editingBooking ? editingBooking._id : 'no-admin-edit',
        open: !!editingBooking,
        booking: editingBooking,
        basket: basket,
        isAdmin: true,
        adminPassword: adminPw,
        onClose: function() { setEditingBooking(null); },
        onSubmit: submitAdminEditBooking
      }),
      bookings.length === 0
        ? h('p', { className: 'pl-hint' }, 'Aucune réservation pour ce panier.')
        : h('div', { className: 'pl-bookings-list' },
            bookings.map(function(bk) {
              return h('div', { key: bk._id, className: 'pl-booking-card' },
                h('div', { className: 'pl-booking-card__header' },
                  h('span', { className: 'pl-booking-card__name' }, helpers.escapeHtml(bk.name)),
                  h('span', { className: 'pl-booking-card__date' }, helpers.formatDateTime(bk.createdAt))
                ),
                h('div', { className: 'pl-booking-card__contact' },
                  bk.email ? h('span', null, '📧 ' + helpers.escapeHtml(bk.email)) : null,
                  bk.phone ? h('span', null, '📱 ' + helpers.escapeHtml(bk.phone)) : null
                ),
                h('div', { className: 'pl-booking-card__items' },
                  bk.items.map(function(it) {
                    return h('span', { key: it.key, className: 'pl-tag' }, helpers.escapeHtml(it.key) + ' ×' + it.quantity);
                  })
                ),
                bk.extraNotes ? h('div', { className: 'pl-booking-card__notes' }, '📝 ' + helpers.escapeHtml(bk.extraNotes)) : null,
                h('div', { className: 'pl-booking-card__actions' },
                  h('button', { className: 'pl-btn pl-btn--small', onClick: function() { setEditingBooking(bk); } }, '✎ Modifier'),
                  h('button', { className: 'pl-btn pl-btn--danger pl-btn--small', onClick: function() { deleteBooking(bk._id); } }, 'Annuler')
                )
              );
            })
          )
    );
  }

  // ═══════════════════════════════════════════════════════════════════
  // Main App
  // ═══════════════════════════════════════════════════════════════════
  function App() {
    var providersState = useState([]);
    var providers = providersState[0]; var setProviders = providersState[1];

    var basketsState = useState([]);
    var baskets = basketsState[0]; var setBaskets = basketsState[1];

    var filterState = useState(helpers.loadFilterProvider());
    var filterProvider = filterState[0]; var setFilterProvider = filterState[1];

    var viewState = useState('browse'); // browse | admin
    var view = viewState[0]; var setView = viewState[1];

    var adminProviderState = useState(null);
    var adminProvider = adminProviderState[0]; var setAdminProvider = adminProviderState[1];

    var adminPwState = useState('');
    var adminPw = adminPwState[0]; var setAdminPw = adminPwState[1];

    var adminBasketsState = useState([]);
    var adminBaskets = adminBasketsState[0]; var setAdminBaskets = adminBasketsState[1];

    var showProviderModalState = useState(false);
    var showProviderModal = showProviderModalState[0]; var setShowProviderModal = showProviderModalState[1];

    var showBasketModalState = useState(false);
    var showBasketModal = showBasketModalState[0]; var setShowBasketModal = showBasketModalState[1];

    var showBookingModalState = useState(false);
    var showBookingModal = showBookingModalState[0]; var setShowBookingModal = showBookingModalState[1];

    var showLookupModalState = useState(false);
    var showLookupModal = showLookupModalState[0]; var setShowLookupModal = showLookupModalState[1];

    var showAdminLoginState = useState(false);
    var showAdminLogin = showAdminLoginState[0]; var setShowAdminLogin = showAdminLoginState[1];

    var selectedBasketState = useState(null);
    var selectedBasket = selectedBasketState[0]; var setSelectedBasket = selectedBasketState[1];

    var editingProviderState = useState(null);
    var editingProvider = editingProviderState[0]; var setEditingProvider = editingProviderState[1];

    var editingBasketState = useState(null);
    var editingBasket = editingBasketState[0]; var setEditingBasket = editingBasketState[1];

    var selectedAdminBasketState = useState(null);
    var selectedAdminBasket = selectedAdminBasketState[0]; var setSelectedAdminBasket = selectedAdminBasketState[1];

    // ─── Load data ──────────────────────────────────────────────────────
    function loadProviders() {
      fetch(API_BASE + '/providers')
        .then(function(r) { return r.json(); })
        .then(function(data) { setProviders(data.providers || []); })
        .catch(function() { helpers.showToast('Erreur de chargement', 'error'); });
    }

    function loadBaskets(providerId) {
      var url = API_BASE + '/baskets';
      if (providerId) url += '?providerId=' + providerId;
      fetch(url)
        .then(function(r) { return r.json(); })
        .then(function(data) { setBaskets(data.baskets || []); })
        .catch(function() { helpers.showToast('Erreur de chargement', 'error'); });
    }

    function loadAdminBaskets(providerId, pw) {
      fetch(API_BASE + '/baskets?providerId=' + providerId)
        .then(function(r) { return r.json(); })
        .then(function(data) { setAdminBaskets(data.baskets || []); })
        .catch(function() { helpers.showToast('Erreur de chargement', 'error'); });
    }

    useEffect(function() { loadProviders(); loadBaskets(filterProvider); }, []);

    useEffect(function() {
      helpers.saveFilterProvider(filterProvider);
      loadBaskets(filterProvider);
    }, [filterProvider]);

    // ─── Provider CRUD ──────────────────────────────────────────────────
    function createProvider(data) {
      fetch(API_BASE + '/providers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function(p) {
          helpers.showToast('Fournisseur créé ✓');
          setShowProviderModal(false);
          loadProviders();
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    function updateProvider(id, data) {
      fetch(API_BASE + '/providers/' + id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Fournisseur mis à jour ✓');
          setShowProviderModal(false);
          setEditingProvider(null);
          loadProviders();
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    function deleteProvider(id, pw) {
      if (!confirm('Supprimer ce fournisseur et tous ses paniers/réservations ?')) return;
      fetch(API_BASE + '/providers/' + id, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ adminPassword: pw })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Fournisseur supprimé ✓');
          helpers.clearAdminSession(id);
          setView('browse');
          setAdminProvider(null);
          loadProviders();
          loadBaskets(filterProvider);
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    // ─── Basket CRUD ────────────────────────────────────────────────────
    function createBasket(data) {
      fetch(API_BASE + '/baskets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Panier créé ✓');
          setShowBasketModal(false);
          loadBaskets(filterProvider);
          if (view === 'admin') loadAdminBaskets(adminProvider._id, adminPw);
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    function updateBasket(id, data) {
      fetch(API_BASE + '/baskets/' + id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Panier mis à jour ✓');
          setShowBasketModal(false);
          setEditingBasket(null);
          loadBaskets(filterProvider);
          if (view === 'admin') loadAdminBaskets(adminProvider._id, adminPw);
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    function deleteBasket(id, pw) {
      if (!confirm('Supprimer ce panier et toutes ses réservations ?')) return;
      fetch(API_BASE + '/baskets/' + id, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ adminPassword: pw })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Panier supprimé ✓');
          setSelectedAdminBasket(null);
          loadBaskets(filterProvider);
          if (view === 'admin') loadAdminBaskets(adminProvider._id, adminPw);
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    // ─── Booking ────────────────────────────────────────────────────────
    function createBooking(basketId, data) {
      fetch(API_BASE + '/baskets/' + basketId + '/bookings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error); });
          return r.json();
        })
        .then(function() {
          helpers.showToast('Réservation confirmée ✓');
          setShowBookingModal(false);
          setSelectedBasket(null);
          loadBaskets(filterProvider); // refresh availability
        })
        .catch(function(e) { helpers.showToast(e.message, 'error'); });
    }

    // ─── Admin map marker ───────────────────────────────────────────────
    function createMapMarker(providerId, pw) {
      fetch(API_BASE + '/providers/' + providerId + '/map-marker', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ adminPassword: pw })
      })
        .then(function(r) { return r.json(); })
        .then(function(data) {
          if (data.markerUrl) {
            helpers.showToast('Marqueur carte créé ✓');
            window.open(data.markerUrl, '_blank');
          } else {
            helpers.showToast('Carte interactive non disponible', 'error');
          }
        })
        .catch(function() { helpers.showToast('Erreur', 'error'); });
    }

    // ─── Admin entry point ──────────────────────────────────────────────
    function enterAdmin(provider) {
      var savedPw = helpers.getAdminPassword(provider._id);
      if (savedPw) {
        // Verify saved password still works
        fetch(API_BASE + '/providers/' + provider._id + '/verify', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ adminPassword: savedPw })
        })
          .then(function(r) {
            if (r.ok) {
              setAdminProvider(provider);
              setAdminPw(savedPw);
              setView('admin');
              loadAdminBaskets(provider._id, savedPw);
            } else {
              helpers.clearAdminSession(provider._id);
              setAdminProvider(provider);
              setShowAdminLogin(true);
            }
          });
      } else {
        setAdminProvider(provider);
        setShowAdminLogin(true);
      }
    }

    function onAdminVerified(pw) {
      helpers.setAdminPassword(adminProvider._id, pw);
      setAdminPw(pw);
      setShowAdminLogin(false);
      setView('admin');
      loadAdminBaskets(adminProvider._id, pw);
    }

    // ─── Get provider name by ID ────────────────────────────────────────
    function getProviderName(providerId) {
      var p = providers.find(function(pr) { return pr._id === providerId; });
      return p ? p.name : '';
    }

    // ═══════════════════════════════════════════════════════════════════
    // RENDER
    // ═══════════════════════════════════════════════════════════════════

    // ─── Navbar ──────────────────────────────────────────────────────────
    var navbar = h('nav', { className: 'pl-navbar' },
      h('div', { className: 'pl-navbar__left' },
        h('a', { href: '/', className: 'pl-navbar__home' }, '🏠'),
        h('span', { className: 'pl-navbar__title' }, '🧺 Panier Libre')
      ),
      h('div', { className: 'pl-navbar__right' },
        h('button', {
          className: 'pl-btn pl-btn--small',
          onClick: function() { setShowLookupModal(true); }
        }, '📋 Mes réservations'),
        view === 'admin' ? h('button', {
          className: 'pl-btn pl-btn--small',
          onClick: function() { setView('browse'); setAdminProvider(null); setSelectedAdminBasket(null); }
        }, '← Retour') : null
      )
    );

    // ─── Browse view ────────────────────────────────────────────────────
    var browseView = h('div', { className: 'pl-browse' },
      // Provider filter
      h('div', { className: 'pl-filter-bar' },
        h('select', {
          className: 'pl-select',
          value: filterProvider,
          onChange: function(e) { setFilterProvider(e.target.value); }
        },
          h('option', { value: '' }, 'Tous les fournisseurs'),
          providers.map(function(p) {
            return h('option', { key: p._id, value: p._id }, helpers.escapeHtml(p.name));
          })
        ),
        h('button', {
          className: 'pl-btn pl-btn--small',
          onClick: function() { setShowProviderModal(true); setEditingProvider(null); }
        }, '+ Fournisseur')
      ),
      // Baskets grid
      baskets.length === 0
        ? h('div', { className: 'pl-empty' },
            h('div', { className: 'pl-empty__icon' }, '🧺'),
            h('p', null, 'Aucun panier disponible pour le moment.'),
            h('p', { className: 'pl-hint' }, 'Les fournisseurs peuvent créer des paniers via l\'espace admin.')
          )
        : h('div', { className: 'pl-baskets-grid' },
            baskets.map(function(basket) {
              var pName = getProviderName(basket.providerId);
              var canBook = basket.status === 'ongoing' || basket.status === 'incoming';
              return h('div', { key: basket._id, className: 'pl-basket-card' },
                h('div', { className: 'pl-basket-card__header' },
                  h(StatusBadge, { status: basket.status }),
                  pName ? h('span', { className: 'pl-basket-card__provider' }, helpers.escapeHtml(pName)) : null
                ),
                h('h3', { className: 'pl-basket-card__title' }, helpers.escapeHtml(basket.title)),
                basket.description ? h('p', { className: 'pl-basket-card__desc' }, helpers.escapeHtml(basket.description).slice(0, 150) + (basket.description.length > 150 ? '…' : '')) : null,
                h('div', { className: 'pl-basket-card__items' },
                  basket.items.map(function(it) {
                    return h('span', {
                      key: it.key,
                      className: 'pl-tag' + (it.available === 0 ? ' pl-tag--empty' : '')
                    }, helpers.escapeHtml(it.key) + ' (' + it.available + '/' + it.quantity + ')');
                  })
                ),
                h('div', { className: 'pl-basket-card__dates' },
                  h('span', null, '📅 ' + helpers.formatDate(basket.startDate)),
                  basket.endDate ? h('span', null, ' → ' + helpers.formatDate(basket.endDate)) : null
                ),
                canBook
                  ? h('button', {
                      className: 'pl-btn pl-btn--primary',
                      onClick: function() { setSelectedBasket(basket); setShowBookingModal(true); }
                    }, 'Réserver')
                  : h('span', { className: 'pl-basket-card__closed' }, basket.status === 'finished' ? 'Terminé' : 'Indisponible'),
                // Admin entry button
                h('button', {
                  className: 'pl-btn pl-btn--small pl-btn--admin',
                  onClick: function() {
                    var fullProvider = providers.find(function(pr) { return pr._id === basket.providerId; });
                    enterAdmin(fullProvider || { _id: basket.providerId, name: pName });
                  },
                  title: 'Espace admin'
                }, '⚙ Admin')
              );
            })
          ),
      // Providers list
      providers.length > 0 ? h('div', { className: 'pl-providers-section' },
        h('h3', { className: 'pl-section-title' }, 'Fournisseurs'),
        providers.map(function(p) {
          return h('div', { key: p._id, className: 'pl-provider-card' },
            h('div', { className: 'pl-provider-card__info' },
              h('span', { className: 'pl-provider-card__name' }, helpers.escapeHtml(p.name)),
              p.description ? h('span', { className: 'pl-provider-card__desc' }, helpers.escapeHtml(p.description).slice(0, 100)) : null,
              p.address ? h('span', { className: 'pl-provider-card__addr' }, '📍 ' + helpers.escapeHtml(p.address)) : null
            ),
            h('div', { className: 'pl-provider-card__actions' },
              h('button', {
                className: 'pl-btn pl-btn--small',
                onClick: function() { setFilterProvider(p._id); }
              }, 'Voir paniers'),
              h('button', {
                className: 'pl-btn pl-btn--small pl-btn--admin',
                onClick: function() { enterAdmin(p); }
              }, '⚙ Admin')
            )
          );
        })
      ) : null
    );

    // ─── Admin view ─────────────────────────────────────────────────────
    var adminView = h('div', { className: 'pl-admin' },
      h('div', { className: 'pl-admin__header' },
        h('h2', { className: 'pl-admin__provider-name' }, adminProvider ? '⚙ ' + helpers.escapeHtml(adminProvider.name) : 'Admin'),
        adminProvider ? h('div', { className: 'pl-admin__actions' },
          h('button', {
            className: 'pl-btn pl-btn--small',
            onClick: function() {
              setEditingProvider(adminProvider);
              setShowProviderModal(true);
            }
          }, '✎ Modifier'),
          adminProvider.address || (adminProvider.location && adminProvider.location.lat)
            ? h('button', {
                className: 'pl-btn pl-btn--small',
                onClick: function() { createMapMarker(adminProvider._id, adminPw); }
              }, '🗺 Carte')
            : null,
          h('button', {
            className: 'pl-btn pl-btn--small pl-btn--danger',
            onClick: function() { deleteProvider(adminProvider._id, adminPw); }
          }, '🗑 Supprimer')
        ) : null
      ),
      // Create basket button
      h('button', {
        className: 'pl-btn pl-btn--primary',
        onClick: function() {
          setEditingBasket(null);
          setShowBasketModal(true);
        },
        style: { marginBottom: '1rem' }
      }, '+ Nouveau panier'),
      // Admin baskets
      adminBaskets.length === 0
        ? h('p', { className: 'pl-hint' }, 'Aucun panier. Créez votre premier panier !')
        : h('div', { className: 'pl-admin-baskets' },
            adminBaskets.map(function(basket) {
              var isSelected = selectedAdminBasket && selectedAdminBasket._id === basket._id;
              return h('div', { key: basket._id, className: 'pl-admin-basket' + (isSelected ? ' pl-admin-basket--selected' : '') },
                h('div', { className: 'pl-admin-basket__header', onClick: function() { setSelectedAdminBasket(isSelected ? null : basket); } },
                  h('div', null,
                    h(StatusBadge, { status: basket.status }),
                    h('span', { className: 'pl-admin-basket__title' }, helpers.escapeHtml(basket.title))
                  ),
                  h('span', { className: 'pl-admin-basket__dates' }, helpers.formatDate(basket.startDate) + (basket.endDate ? ' → ' + helpers.formatDate(basket.endDate) : ''))
                ),
                isSelected ? h('div', { className: 'pl-admin-basket__detail' },
                  basket.description ? h('p', null, helpers.escapeHtml(basket.description)) : null,
                  h('div', { className: 'pl-basket-card__items' },
                    basket.items.map(function(it) {
                      return h('span', {
                        key: it.key,
                        className: 'pl-tag' + (it.available === 0 ? ' pl-tag--empty' : '')
                      }, helpers.escapeHtml(it.key) + ' (' + it.available + '/' + it.quantity + ')');
                    })
                  ),
                  h('div', { className: 'pl-admin-basket__actions' },
                    h('button', {
                      className: 'pl-btn pl-btn--small',
                      onClick: function() { setEditingBasket(basket); setShowBasketModal(true); }
                    }, '✎ Modifier'),
                    h('button', {
                      className: 'pl-btn pl-btn--small pl-btn--danger',
                      onClick: function() { deleteBasket(basket._id, adminPw); }
                    }, '🗑 Supprimer')
                  ),
                  h(AdminBookingsView, { basket: basket, adminPassword: adminPw })
                ) : null
              );
            })
          )
    );

    // ─── Assemble ───────────────────────────────────────────────────────
    return h('div', { className: 'pl-app' },
      navbar,
      h('main', { className: 'pl-main' },
        view === 'browse' ? browseView : adminView
      ),

      // Modals (key prop forces remount so state resets between opens)
      h(ProviderModal, {
        key: editingProvider ? editingProvider._id : 'new-provider',
        open: showProviderModal,
        provider: editingProvider,
        onClose: function() { setShowProviderModal(false); setEditingProvider(null); },
        onSubmit: function(data) {
          if (editingProvider) {
            updateProvider(editingProvider._id, Object.assign({}, data, { adminPassword: adminPw || helpers.getAdminPassword(editingProvider._id) }));
          } else {
            createProvider(data);
          }
        }
      }),
      h(BasketModal, {
        key: editingBasket ? editingBasket._id : 'new-basket',
        open: showBasketModal,
        basket: editingBasket,
        providerId: adminProvider ? adminProvider._id : '',
        adminPassword: adminPw,
        onClose: function() { setShowBasketModal(false); setEditingBasket(null); },
        onSubmit: function(data) {
          if (editingBasket) {
            updateBasket(editingBasket._id, data);
          } else {
            createBasket(data);
          }
        }
      }),
      h(BookingModal, {
        key: selectedBasket ? selectedBasket._id : 'new-booking',
        open: showBookingModal,
        basket: selectedBasket,
        onClose: function() { setShowBookingModal(false); setSelectedBasket(null); },
        onSubmit: function(data) { createBooking(selectedBasket._id, data); }
      }),
      h(LookupModal, {
        open: showLookupModal,
        onClose: function() { setShowLookupModal(false); }
      }),
      h(AdminLoginModal, {
        open: showAdminLogin,
        providerId: adminProvider ? adminProvider._id : '',
        onClose: function() { setShowAdminLogin(false); setAdminProvider(null); },
        onVerified: onAdminVerified
      })
    );
  }

  return { App: App };
})();
