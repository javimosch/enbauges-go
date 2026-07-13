window.InteractiveMapApp = (function() {
  var h = React.createElement;
  var useState = React.useState;
  var useEffect = React.useEffect;
  var useRef = React.useRef;
  var helpers = window.InteractiveMapHelpers;
  var MarkerModal = window.InteractiveMapModal;

  var API_BASE = '/carte-interactive/api';

  function createMarkerIcon(category) {
    return L.divIcon({
      className: 'imap-marker-icon imap-marker--' + category,
      iconSize: [22, 22],
      iconAnchor: [11, 11],
      popupAnchor: [0, -11],
      html: ''
    });
  }

  function App() {
    var mapRef = useRef(null);
    var markersLayerRef = useRef(null);
    var tempMarkerRef = useRef(null);
    var addModeHandlerRef = useRef(null);
    var leafletMarkerRefs = useRef({});

    var markersState = useState([]);
    var markers = markersState[0];
    var setMarkers = markersState[1];

    var filtersState = useState({});
    var activeFilters = filtersState[0];
    var setActiveFilters = filtersState[1];

    var modalState = useState({ open: false, isEdit: false, editingId: null });
    var modal = modalState[0];
    var setModal = modalState[1];

    var formState = useState({
      title: '',
      description: '',
      category: 'lieu',
      location: { lat: null, lng: null, address: '' },
      contact: '',
      tagsInput: ''
    });
    var form = formState[0];
    var setForm = formState[1];

    var submitState = useState(false);
    var submitting = submitState[0];
    var setSubmitting = submitState[1];

    var errorState = useState('');
    var formError = errorState[0];
    var setFormError = errorState[1];

    var hintState = useState('');
    var hint = hintState[0];
    var setHint = hintState[1];

    var listState = useState(false);
    var showList = listState[0];
    var setShowList = listState[1];

    var searchState = useState('');
    var searchQuery = searchState[0];
    var setSearchQuery = searchState[1];

    var skipRenderRef = useRef(false);
    var prevSearchRef = useRef('');

    var labelsState = useState(false);
    var showLabels = labelsState[0];
    var setShowLabels = labelsState[1];

    var creatorId = helpers.getCreatorId();

    // Initialize map
    useEffect(function() {
      if (mapRef.current) return;

      var map = L.map('map', {
        zoomControl: true
      }).setView(helpers.BAUGES_CENTER, helpers.BAUGES_ZOOM);

      L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
        maxZoom: 19
      }).addTo(map);

      var markersLayer = L.layerGroup().addTo(map);
      markersLayerRef.current = markersLayer;
      mapRef.current = map;

      // Initial load (the [activeFilters] effect is gated on mapRef.current, so this is the primary load)
      loadMarkers();

      return function() {
        map.remove();
        mapRef.current = null;
      };
    }, []);

    // Toggle filter
    function toggleFilter(category) {
      setActiveFilters(function(prev) {
        var next = Object.assign({}, prev);
        if (next[category]) {
          delete next[category];
        } else {
          next[category] = true;
        }
        return next;
      });
    }

    // Load markers from API
    function loadMarkers() {
      var params = new URLSearchParams();
      var filterKeys = Object.keys(activeFilters);
      if (filterKeys.length > 0) {
        params.set('category', filterKeys.join(','));
      }

      fetch(API_BASE + '?' + params.toString())
        .then(function(r) { return r.json(); })
        .then(function(data) {
          var mList = data.markers || [];
          setMarkers(mList);
        })
        .catch(function(e) {
          console.error('[interactive-map] Failed to load markers:', e);
        });
    }

    // Render markers on map
    function renderMarkers(mList) {
      if (!markersLayerRef.current) return;
      markersLayerRef.current.clearLayers();
      leafletMarkerRefs.current = {};

      mList.forEach(function(marker) {
        var icon = createMarkerIcon(marker.category);
        if (showLabels) {
          var label = marker.title.length > 18 ? marker.title.slice(0, 16) + '…' : marker.title;
          icon.options.html = '<span class="imap-marker-label">' + escapeHtml(label) + '</span>';
        }
        var m = L.marker([marker.location.lat, marker.location.lng], {
          icon: icon,
          draggable: true
        });

        var isCreator = marker.creatorId === creatorId;
        var myConfirms = helpers.getMyConfirmations();
        var isConfirmed = myConfirms.includes(marker._id);

        var popupHtml = '<div class="imap-popup">' +
          '<span class="imap-popup__category imap-popup__category--' + marker.category + '">' + helpers.getCategoryLabel(marker.category) + '</span>' +
          '<div class="imap-popup__title">' + escapeHtml(marker.title) + '</div>' +
          (marker.description ? '<div class="imap-popup__desc">' + escapeHtml(marker.description) + '</div>' : '') +
          (marker.location.address ? '<div class="imap-popup__meta"><span>📍 ' + escapeHtml(marker.location.address) + '</span></div>' : '') +
          '<div class="imap-popup__hint">↕ Glissez le marqueur pour repositionner</div>' +
          (marker.contact ? '<div class="imap-popup__meta"><span>✉ ' + escapeHtml(marker.contact) + '</span></div>' : '') +
          '<div class="imap-popup__meta"><span>il y a ' + helpers.timeAgo(marker.createdAt) + '</span><span>👍 ' + (marker.confirmations ? marker.confirmations.length : 0) + '</span></div>' +
          '<button class="imap-popup__confirm' + (isConfirmed ? ' imap-popup__confirm--confirmed' : '') + '" data-marker-id="' + marker._id + '">' +
            (isConfirmed ? '✓ Confirmé' : '👍 Confirmer') +
          '</button>' +
          (isCreator ? '<button class="imap-popup__confirm" data-delete-id="' + marker._id + '" style="border-color:#DC2626;color:#DC2626;margin-left:0.375rem;">Supprimer</button>' : '') +
        '</div>';

        m.bindPopup(popupHtml, { maxWidth: 280 });

        // Handle drag to reposition
        m.on('dragend', function(e) {
          var newPos = e.target.getLatLng();
          repositionMarker(marker._id, newPos.lat, newPos.lng);
        });

        markersLayerRef.current.addLayer(m);
        leafletMarkerRefs.current[marker._id] = m;
      });
    }

    function escapeHtml(text) {
      if (!text) return '';
      var div = document.createElement('div');
      div.textContent = text;
      return div.innerHTML;
    }

    // Handle popup button clicks via event delegation
    useEffect(function() {
      var mapEl = document.getElementById('map');
      if (!mapEl) return;

      function handleClick(e) {
        var target = e.target;
        if (target.dataset && target.dataset.markerId) {
          confirmMarker(target.dataset.markerId);
        }
        if (target.dataset && target.dataset.deleteId) {
          deleteMarker(target.dataset.deleteId);
        }
      }

      mapEl.addEventListener('click', handleClick);
      return function() { mapEl.removeEventListener('click', handleClick); };
    }, [markers]);

    // Reload when filters change
    useEffect(function() {
      if (mapRef.current) loadMarkers();
    }, [activeFilters]);

    // Re-render markers when data, search query, or label toggle changes
    useEffect(function() {
      if (skipRenderRef.current) {
        skipRenderRef.current = false;
        return;
      }
      if (mapRef.current) renderMarkers(filteredMarkers);
    }, [markers, searchQuery, showLabels]);

    // Confirm marker
    function confirmMarker(markerId) {
      var voterId = creatorId;
      fetch(API_BASE + '/' + markerId + '/confirm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ voterId: voterId })
      })
        .then(function(r) { return r.json(); })
        .then(function(data) {
          var myConfirms = helpers.getMyConfirmations();
          if (data.confirmed) {
            myConfirms.push(markerId);
          } else {
            myConfirms = myConfirms.filter(function(id) { return id !== markerId; });
          }
          helpers.setMyConfirmations(myConfirms);
          loadMarkers();
          helpers.showToast(data.confirmed ? 'Confirmé !' : 'Confirmation retirée');
        })
        .catch(function(e) {
          helpers.showToast('Erreur', 'error');
        });
    }

    // Delete marker
    function deleteMarker(markerId) {
      if (!confirm('Supprimer ce marqueur ?')) return;
      fetch(API_BASE + '/' + markerId, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ creatorId: creatorId })
      })
        .then(function(r) {
          if (!r.ok) throw new Error('Erreur');
          loadMarkers();
          helpers.showToast('Marqueur supprimé');
        })
        .catch(function(e) {
          helpers.showToast('Erreur lors de la suppression', 'error');
        });
    }

    // Handle add mode via button
    function startAddMode() {
      // Clean up any previous handler
      if (addModeHandlerRef.current && mapRef.current) {
        mapRef.current.off('click', addModeHandlerRef.current);
        addModeHandlerRef.current = null;
      }

      setHint('Cliquez sur la carte pour placer votre marqueur');
      mapRef.current.getContainer().style.cursor = 'crosshair';

      // One-click handler to place marker
      var handler = function(e) {
        mapRef.current.off('click', handler);
        mapRef.current.getContainer().style.cursor = '';
        addModeHandlerRef.current = null;

        var lat = e.latlng.lat;
        var lng = e.latlng.lng;

        if (tempMarkerRef.current) {
          mapRef.current.removeLayer(tempMarkerRef.current);
        }

        var tempMarker = L.marker([lat, lng], {
          icon: createMarkerIcon('lieu'),
          opacity: 0.6
        }).addTo(mapRef.current);
        tempMarkerRef.current = tempMarker;

        setForm(function(prev) {
          return Object.assign({}, prev, {
            location: { lat: lat, lng: lng, address: '' }
          });
        });

        setModal({ open: true, isEdit: false, editingId: null });

        helpers.reverseGeocode(lat, lng, function(address) {
          if (address) {
            setForm(function(prev) {
              var loc = Object.assign({}, prev.location, { address: address });
              return Object.assign({}, prev, { location: loc });
            });
          }
        });

        setTimeout(function() { setHint(''); }, 3000);
      };

      addModeHandlerRef.current = handler;
      mapRef.current.on('click', handler);
    }

    // Submit form
    function handleSubmit() {
      setFormError('');

      if (!form.title.trim()) {
        setFormError('Le titre est requis');
        return;
      }
      if (!form.category) {
        setFormError('La catégorie est requise');
        return;
      }
      if (!form.location.lat || !form.location.lng) {
        setFormError('Cliquez sur la carte pour placer le marqueur');
        return;
      }

      setSubmitting(true);

      var tags = (form.tagsInput || '').split(',').map(function(t) { return t.trim(); }).filter(function(t) { return t; });

      var payload = {
        title: form.title.trim(),
        description: form.description ? form.description.trim() : undefined,
        category: form.category,
        location: {
          lat: form.location.lat,
          lng: form.location.lng,
          address: form.location.address ? form.location.address.trim() : undefined
        },
        contact: form.contact ? form.contact.trim() : undefined,
        tags: tags,
        creatorId: creatorId
      };

      var url = modal.isEdit && modal.editingId ? API_BASE + '/' + modal.editingId : API_BASE;
      var method = modal.isEdit ? 'PUT' : 'POST';

      fetch(url, {
        method: method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error || 'Erreur'); });
          return r.json();
        })
        .then(function(data) {
          setSubmitting(false);
          closeModal();
          loadMarkers();
          helpers.showToast(modal.isEdit ? 'Marqueur modifié' : 'Marqueur ajouté !');

          // Remove temp marker
          if (tempMarkerRef.current && mapRef.current) {
            mapRef.current.removeLayer(tempMarkerRef.current);
            tempMarkerRef.current = null;
          }
        })
        .catch(function(e) {
          setSubmitting(false);
          setFormError(e.message);
        });
    }

    // Close modal
    function closeModal() {
      setModal({ open: false, isEdit: false, editingId: null });
      setForm({
        title: '',
        description: '',
        category: 'lieu',
        location: { lat: null, lng: null, address: '' },
        contact: '',
        tagsInput: ''
      });
      setFormError('');
      setHint('');

      // Clean up add-mode handler & cursor
      if (addModeHandlerRef.current && mapRef.current) {
        mapRef.current.off('click', addModeHandlerRef.current);
        addModeHandlerRef.current = null;
        mapRef.current.getContainer().style.cursor = '';
      }

      if (tempMarkerRef.current && mapRef.current) {
        mapRef.current.removeLayer(tempMarkerRef.current);
        tempMarkerRef.current = null;
      }
    }

    // Reposition marker (drag)
    function repositionMarker(markerId, lat, lng) {
      fetch(API_BASE + '/' + markerId + '/reposition', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ lat: lat, lng: lng, voterId: creatorId })
      })
        .then(function(r) {
          if (!r.ok) return r.json().then(function(e) { throw new Error(e.error || 'Erreur'); });
          return r.json();
        })
        .then(function(data) {
          helpers.showToast('Position mise à jour ✓');
          // Skip full re-render — Leaflet marker is already at the correct position
          skipRenderRef.current = true;
          // Update local markers state without full reload
          setMarkers(function(prev) {
            return prev.map(function(m) {
              if (m._id === markerId) {
                return Object.assign({}, m, { location: Object.assign({}, m.location, { lat: lat, lng: lng }) });
              }
              return m;
            });
          });
          // Close popup so it re-renders with fresh data on next click
          var lm = leafletMarkerRefs.current[markerId];
          if (lm) lm.closePopup();
        })
        .catch(function(e) {
          helpers.showToast('Erreur de repositionnement', 'error');
          // Revert: reload markers to reset position
          loadMarkers();
        });
    }

    // Fly to marker and open its popup
    function flyToMarker(markerId) {
      var lm = leafletMarkerRefs.current[markerId];
      if (!lm || !mapRef.current) return;
      var latlng = lm.getLatLng();
      mapRef.current.flyTo(latlng, 15, { duration: 0.8 });
      setTimeout(function() { lm.openPopup(); }, 850);
      // Close list on mobile
      if (window.innerWidth < 640) setShowList(false);
    }

    // Handle keyboard
    useEffect(function() {
      function handleKey(e) {
        if (e.key === 'Escape') {
          if (modal.open) {
            closeModal();
          } else if (showList) {
            setShowList(false);
          }
        }
      }
      window.addEventListener('keydown', handleKey);
      return function() { window.removeEventListener('keydown', handleKey); };
    }, [modal.open, showList]);

    var categories = ['lieu', 'service', 'evenement', 'autre'];

    // Compute filtered markers based on search query
    var filteredMarkers = markers;
    if (searchQuery.trim()) {
      var q = searchQuery.trim().toLowerCase();
      filteredMarkers = markers.filter(function(m) {
        return (m.title && m.title.toLowerCase().indexOf(q) !== -1) ||
          (m.description && m.description.toLowerCase().indexOf(q) !== -1) ||
          (m.location && m.location.address && m.location.address.toLowerCase().indexOf(q) !== -1) ||
          (m.tags && m.tags.some(function(t) { return t.toLowerCase().indexOf(q) !== -1; }));
      });
    }
    var markerCount = filteredMarkers.length;

    // Auto-open list when search yields results; close when cleared
    useEffect(function() {
      if (searchQuery.trim() && !prevSearchRef.current.trim() && filteredMarkers.length > 0) {
        setShowList(true);
      } else if (!searchQuery.trim() && prevSearchRef.current.trim()) {
        setShowList(false);
      }
      prevSearchRef.current = searchQuery;
    }, [searchQuery, filteredMarkers.length]);

    return h('div', null,
      // Navbar
      h('nav', { className: 'imap-navbar' },
        h('a', { href: '/', className: 'imap-navbar__brand' },
          h('svg', { viewBox: '0 0 32 32', fill: 'none' },
            h('circle', { cx: 16, cy: 16, r: 14, fill: '#7A9E7E' }),
            h('path', { d: 'M16 8C12 8 8 12 8 16C8 20 12 24 16 24C20 24 24 20 24 16', stroke: '#FAF7F2', strokeWidth: 2, strokeLinecap: 'round' }),
            h('path', { d: 'M16 8C20 8 24 12 24 16C24 20 20 24 16 24', stroke: '#D4A03C', strokeWidth: 2, strokeLinecap: 'round' }),
            h('circle', { cx: 16, cy: 16, r: 3, fill: '#FAF7F2' })
          ),
          h('span', null, 'Enbauges')
        ),
        h('div', { className: 'imap-navbar__actions' },
          h('a', { href: '/', className: 'enbauges-btn-secondary', style: { textDecoration: 'none', fontSize: '0.875rem', padding: '0.5rem 1rem' } }, '← Canvas'),
          h('span', { style: { fontSize: '0.8125rem', color: 'var(--enbauges-brown)', fontWeight: 500 } }, markerCount + ' marqueur' + (markerCount !== 1 ? 's' : ''))
        )
      ),

      // Map container
      h('div', { style: { position: 'relative' } },
        h('div', { id: 'map' }),

        // Search + Filter chips
        h('div', { className: 'imap-filters' },
          h('div', { className: 'imap-search' },
            h('svg', { className: 'imap-search__icon', width: 16, height: 16, viewBox: '0 0 16 16', fill: 'none' },
              h('circle', { cx: 7, cy: 7, r: 5.5, stroke: 'currentColor', strokeWidth: 1.5 }),
              h('path', { d: 'M11 11l3.5 3.5', stroke: 'currentColor', strokeWidth: 1.5, strokeLinecap: 'round' })
            ),
            h('input', {
              type: 'text',
              className: 'imap-search__input',
              placeholder: 'Rechercher…',
              value: searchQuery,
              onChange: function(e) { setSearchQuery(e.target.value); },
              'aria-label': 'Rechercher un marqueur'
            }),
            searchQuery ? h('button', {
              className: 'imap-search__clear',
              onClick: function() { setSearchQuery(''); },
              'aria-label': 'Effacer la recherche'
            },
              h('svg', { width: 14, height: 14, viewBox: '0 0 14 14', fill: 'none' },
                h('path', { d: 'M3 3l8 8M11 3l-8 8', stroke: 'currentColor', strokeWidth: 1.5, strokeLinecap: 'round' })
              )
            ) : null
          ),
          categories.map(function(cat) {
            var isActive = !!activeFilters[cat];
            return h('button', {
              key: cat,
              className: 'imap-filter-chip' + (isActive ? ' imap-filter-chip--active imap-filter-chip--' + cat : ''),
              onClick: function() { toggleFilter(cat); }
            }, helpers.getCategoryIcon(cat) + ' ' + helpers.getCategoryLabel(cat));
          })
        ),

        // List toggle button
        h('button', { className: 'imap-list-btn' + (showList ? ' imap-list-btn--active' : ''), onClick: function() { setShowList(function(v) { return !v; }); }, 'aria-label': 'Liste des marqueurs' },
          h('svg', { width: 18, height: 18, viewBox: '0 0 18 18', fill: 'none' },
            h('path', { d: 'M2 4h14M2 9h14M2 14h14', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' })
          ),
          ' Liste'
        ),

        // Add button
        h('button', { className: 'imap-add-btn', onClick: startAddMode },
          h('svg', { width: 18, height: 18, viewBox: '0 0 18 18', fill: 'none' },
            h('path', { d: 'M9 3v12M3 9h12', stroke: 'white', strokeWidth: 2.5, strokeLinecap: 'round' })
          ),
          ' Ajouter'
        ),

        // Labels toggle
        h('button', {
          className: 'imap-labels-btn' + (showLabels ? ' imap-labels-btn--active' : ''),
          onClick: function() { setShowLabels(function(v) { return !v; }); },
          'aria-label': 'Afficher les noms sur la carte',
          title: showLabels ? 'Masquer les noms' : 'Afficher les noms'
        },
          h('svg', { width: 16, height: 16, viewBox: '0 0 16 16', fill: 'none' },
            h('path', { d: 'M2 4h3M2 8h5M2 12h4', stroke: 'currentColor', strokeWidth: 1.5, strokeLinecap: 'round' })
          ),
          ' Noms'
        ),

        // Hint
        hint ? h('div', { className: 'imap-hint' }, hint) : null,

        // Marker list panel
        showList ? h('div', { className: 'imap-list-panel' },
          h('div', { className: 'imap-list-panel__header' },
            h('span', { className: 'imap-list-panel__title' },
              '📍 ' + markerCount + ' marqueur' + (markerCount !== 1 ? 's' : '')
            ),
            h('button', { className: 'imap-list-panel__close', onClick: function() { setShowList(false); }, 'aria-label': 'Fermer' },
              h('svg', { width: 16, height: 16, viewBox: '0 0 16 16', fill: 'currentColor' },
                h('path', { d: 'M4 4l8 8M12 4l-8 8', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round' })
              )
            )
          ),
          filteredMarkers.length === 0
            ? h('div', { className: 'imap-list-panel__empty' }, searchQuery ? 'Aucun résultat pour « ' + searchQuery + ' »' : 'Aucun marqueur trouvé')
            : h('div', { className: 'imap-list-panel__items' },
                filteredMarkers.map(function(m) {
                  return h('button', {
                    key: m._id,
                    className: 'imap-list-item',
                    onClick: function() { flyToMarker(m._id); }
                  },
                    h('div', { className: 'imap-list-item__marker' },
                      h('span', { className: 'imap-list-item__dot imap-list-item__dot--' + m.category }, helpers.getCategoryIcon(m.category))
                    ),
                    h('div', { className: 'imap-list-item__body' },
                      h('div', { className: 'imap-list-item__title' }, m.title),
                      m.description ? h('div', { className: 'imap-list-item__desc' }, m.description) : null,
                      h('div', { className: 'imap-list-item__meta' },
                        h('span', { className: 'imap-list-item__cat imap-list-item__cat--' + m.category }, helpers.getCategoryLabel(m.category)),
                        m.location.address ? h('span', null, ' · ' + m.location.address) : null,
                        h('span', null, ' · 👍 ' + (m.confirmations ? m.confirmations.length : 0))
                      )
                    )
                  );
                })
              )
        ) : null
      ),

      // Modal
      modal.open ? h(MarkerModal, {
        form: form,
        onChange: setForm,
        onSubmit: handleSubmit,
        onClose: closeModal,
        submitting: submitting,
        error: formError,
        isEdit: modal.isEdit
      }) : null
    );
  }

  return App;
})();

// Mount on load
document.addEventListener('DOMContentLoaded', function() {
  var root = ReactDOM.createRoot(document.getElementById('app'));
  root.render(React.createElement(window.InteractiveMapApp));
});
