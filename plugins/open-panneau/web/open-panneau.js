const { createApp, ref, computed, onMounted } = Vue;
createApp({
  setup() {
    const r = (v) => ref(v);
    const municipalities = r([]), announcements = r([]), selectedMunicipality = r(''), selectedType = r(''), loading = r(false);
    const toast = r({ visible: false, message: '', bgClass: '', timer: null, leaving: false });
    const confirm = r({ show: false, message: '', onConfirm: null, btnClass: '' });
    const currentMunicipality = r(null), storedAccessCode = r('');
    const loginEmail = r(''), loginCode = r(''), showLogin = r(false), loginLoading = r(false), loginError = r('');
    const regName = r(''), regEmail = r(''), regCode = r(''), regContactName = r(''), regPhone = r(''), regInsee = r('');
    const showRegister = r(false), regLoading = r(false);
    const postTitle = r(''), postContent = r(''), postType = r('info'), postExpires = r(''), showPost = r(false), postLoading = r(false);
    const myAnnouncements = r([]);
    const isAdmin = r(false), adminMunicipalities = r([]), showAdmin = r(false);
    const adminUsername = r(''), adminPassword = r(''), adminLoading = r(false), adminMunicipalityLoading = r(false);
    const adminAuth = r({ username: '', password: '' }), adminFilter = r(''), adminSearch = r('');
    const adminAnnouncements = r([]), adminAnnLoading = r(false), adminAnnFilterMun = r('');

    const types = [
      { value: 'info', label: 'Information' }, { value: 'alert', label: 'Alerte' },
      { value: 'event', label: 'Événement' }, { value: 'warning', label: 'Attention' }
    ];
    const aboutItems = ['Service 100% gratuit', 'Sans publicité', 'Pas de tracking', 'Données confidentielles'];
    const adminFilters = [
      { value: '', label: 'Toutes' }, { value: 'pending', label: 'En attente' },
      { value: 'active', label: 'Actives' }, { value: 'disabled', label: 'Désactivées' }
    ];

    const filteredAnnouncements = computed(() => {
      let r2 = announcements.value;
      if (selectedMunicipality.value) r2 = r2.filter(a => (a.municipalityId?._id || a.municipalityId) === selectedMunicipality.value);
      if (selectedType.value) r2 = r2.filter(a => a.type === selectedType.value);
      return r2;
    });

    const adminAuthFromLogin = computed(() => currentMunicipality.value && storedAccessCode.value && isAdmin.value);

    const filteredAdminMunicipalities = computed(() => {
      let r2 = adminMunicipalities.value;
      if (adminFilter.value) r2 = r2.filter(m => m.status === adminFilter.value);
      if (adminSearch.value) { const s = adminSearch.value.toLowerCase(); r2 = r2.filter(m => m.name.toLowerCase().includes(s) || m.email.toLowerCase().includes(s)); }
      return r2;
    });

    const adminStatCards = computed(() => {
      const ms = adminMunicipalities.value;
      let p = 0, a = 0, d = 0;
      ms.forEach(m => { p += m.status === 'pending' ? 1 : 0; a += m.status === 'active' ? 1 : 0; d += m.status === 'disabled' ? 1 : 0; });
      return [
        { label: 'Total', value: ms.length, color: 'var(--enbauges-sage)' },
        { label: 'En attente', value: p, color: 'var(--enbauges-amber)' },
        { label: 'Actives', value: a, color: 'var(--enbauges-sage)' },
        { label: 'Désactivées', value: d, color: 'var(--enbauges-terracotta)' }
      ];
    });

    const adminAnnouncementsFiltered = computed(() => {
      let r2 = adminAnnouncements.value;
      if (adminAnnFilterMun.value) r2 = r2.filter(a => (a.municipalityId?._id || a.municipalityId) === adminAnnFilterMun.value);
      return r2;
    });

    const showToast = (message, type = 'success') => {
      if (toast.value.timer) clearTimeout(toast.value.timer);
      const bgMap = { success: 'bg-[var(--enbauges-sage)] text-white', error: 'bg-[var(--enbauges-terracotta)] text-white', info: 'bg-[var(--enbauges-amber)] text-[var(--enbauges-earth)]' };
      toast.value.message = message; toast.value.bgClass = bgMap[type] || bgMap.info;
      toast.value.visible = true; toast.value.leaving = false;
      toast.value.timer = setTimeout(() => dismissToast(), type === 'error' ? 5000 : 3000);
    };
    const dismissToast = () => {
      if (!toast.value.visible) return;
      toast.value.leaving = true;
      setTimeout(() => { toast.value.visible = false; toast.value.leaving = false; }, 200);
      if (toast.value.timer) { clearTimeout(toast.value.timer); toast.value.timer = null; }
    };
    const confirmedAction = (message, fn, btnClass = 'btn-error') => { confirm.value = { message, onConfirm: fn, btnClass, show: true }; };
    const confirmDelete = (id) => confirmedAction('Supprimer cette annonce ?', () => deleteAnnouncement(id));
    const confirmAdminDelete = (id) => confirmedAction('Supprimer cette annonce définitivement ?', () => adminDeleteAnnouncement(id), 'btn-error');

    const api = async (url, opts = {}) => { const r2 = await fetch(url, opts); return { ok: r2.ok, data: await r2.json().catch(() => ({})) }; };

    const fetchMunicipalities = async () => { try { const { data } = await api('/open-panneau/api/municipalities'); municipalities.value = data.municipalities || []; } catch (e) {} };
    const fetchAnnouncements = async () => {
      loading.value = true;
      try {
        const params = new URLSearchParams();
        if (selectedMunicipality.value) params.append('municipalityId', selectedMunicipality.value);
        if (selectedType.value) params.append('type', selectedType.value);
        const { data } = await api('/open-panneau/api/announcements?' + params);
        announcements.value = data.announcements || [];
      } catch (e) { showToast('Erreur de chargement', 'error'); } finally { loading.value = false; }
    };
    const resetFilters = () => { selectedMunicipality.value = ''; selectedType.value = ''; fetchAnnouncements(); };

    const register = async () => {
      if (!regName.value || !regEmail.value || !regCode.value) return showToast('Nom, email et code requis', 'error');
      if (regCode.value.length < 6) return showToast('Code: min 6 caractères', 'error');
      regLoading.value = true;
      try {
        const { ok, data } = await api('/open-panneau/api/municipalities/register', {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: regName.value, email: regEmail.value, accessCode: regCode.value, contactName: regContactName.value, contactPhone: regPhone.value, inseeCode: regInsee.value })
        });
        if (ok) { showToast('Inscription réussie !'); showRegister.value = false; regName.value = regEmail.value = regCode.value = regContactName.value = regPhone.value = regInsee.value = ''; }
        else showToast(data.error || 'Erreur d\'inscription', 'error');
      } catch (e) { showToast('Erreur de connexion', 'error'); } finally { regLoading.value = false; }
    };

    const login = async () => {
      if (!loginEmail.value || !loginCode.value) return showToast('Email et code requis', 'error');
      loginLoading.value = true; loginError.value = '';
      try {
        const { ok, data } = await api('/open-panneau/api/municipalities/auth', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: loginEmail.value, accessCode: loginCode.value })
        });
        if (ok) { currentMunicipality.value = data.municipality; isAdmin.value = data.isAdmin || false; storedAccessCode.value = loginCode.value; showLogin.value = false; loginEmail.value = loginCode.value = ''; showToast('Connexion réussie !'); fetchMyAnnouncements(); }
        else loginError.value = data.error || 'Identifiants invalides';
      } catch (e) { showToast('Erreur de connexion', 'error'); } finally { loginLoading.value = false; }
    };
    const closeLogin = () => { showLogin.value = false; loginError.value = ''; };
    const logout = () => { currentMunicipality.value = null; storedAccessCode.value = ''; isAdmin.value = false; myAnnouncements.value = []; adminMunicipalities.value = []; adminAnnouncements.value = []; showToast('Déconnecté'); };

    const postAnnouncement = async () => {
      if (!postTitle.value || !postContent.value) return showToast('Titre et contenu requis', 'error');
      postLoading.value = true;
      try {
        const { ok, data } = await api('/open-panneau/api/announcements', {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ title: postTitle.value, content: postContent.value, type: postType.value, expiresAt: postExpires.value || null, email: currentMunicipality.value.email, accessCode: storedAccessCode.value })
        });
        if (ok) { showToast('Annonce publiée !'); showPost.value = false; postTitle.value = postContent.value = ''; postType.value = 'info'; postExpires.value = ''; fetchAnnouncements(); fetchMyAnnouncements(); }
        else showToast(data.error || 'Erreur de publication', 'error');
      } catch (e) { showToast('Erreur de connexion', 'error'); } finally { postLoading.value = false; }
    };

    const fetchMyAnnouncements = async () => {
      if (!currentMunicipality.value) return;
      try { const { data } = await api(`/open-panneau/api/announcements?municipalityId=${currentMunicipality.value._id}&includeExpired=true`); myAnnouncements.value = data.announcements || []; } catch (e) {}
    };

    const deleteAnnouncement = async (id) => {
      try {
        const { ok, data } = await api(`/open-panneau/api/announcements/${id}`, {
          method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: currentMunicipality.value.email, accessCode: storedAccessCode.value })
        });
        if (ok) { showToast('Annonce supprimée'); fetchAnnouncements(); fetchMyAnnouncements(); }
        else showToast(data.error || 'Erreur de suppression', 'error');
      } catch (e) { showToast('Erreur de connexion', 'error'); }
    };

    const openAdminPanel = () => { showAdmin.value = true; if (adminAuth.value.username || adminAuthFromLogin.value) { fetchAdminMunicipalities(); fetchAdminAnnouncements(); } };

    const loginAdmin = async () => {
      if (!adminUsername.value || !adminPassword.value) return showToast('Nom d\'utilisateur et mot de passe requis', 'error');
      adminLoading.value = true;
      try {
        const auth = btoa(`${adminUsername.value}:${adminPassword.value}`);
        const { ok, data } = await api('/open-panneau/api/admin/municipalities', { headers: { 'Authorization': `Basic ${auth}` } });
        if (ok) { adminAuth.value = { username: adminUsername.value, password: adminPassword.value }; adminMunicipalities.value = data.municipalities || []; adminUsername.value = adminPassword.value = ''; fetchAdminAnnouncements(); showToast('Connecté en tant que superadmin'); }
        else showToast(data.error || 'Erreur d\'authentification', 'error');
      } catch (e) { showToast('Erreur de connexion', 'error'); } finally { adminLoading.value = false; }
    };

    const logoutAdmin = () => { adminAuth.value = { username: '', password: '' }; adminMunicipalities.value = []; adminAnnouncements.value = []; showToast('Déconnecté'); };

    const getAdminAuthHeaders = () => {
      if (adminAuth.value.username) return { 'Authorization': `Basic ${btoa(`${adminAuth.value.username}:${adminAuth.value.password}`)}` };
      if (currentMunicipality.value && storedAccessCode.value) return { 'X-Admin-Email': currentMunicipality.value.email, 'X-Admin-Code': storedAccessCode.value };
      return {};
    };

    const fetchAdminMunicipalities = async () => {
      adminMunicipalityLoading.value = true;
      try {
        const { ok, data } = await api('/open-panneau/api/admin/municipalities', { headers: getAdminAuthHeaders() });
        if (ok) adminMunicipalities.value = data.municipalities || [];
        else { showToast(data.error || 'Erreur d\'authentification', 'error'); adminMunicipalities.value = []; }
      } catch (e) { showToast('Erreur de connexion', 'error'); } finally { adminMunicipalityLoading.value = false; }
    };

    const updateMunicipalityStatus = async (id, status) => {
      try {
        const { ok, data } = await api(`/open-panneau/api/admin/municipalities/${id}/status`, {
          method: 'PUT', headers: { 'Content-Type': 'application/json', ...getAdminAuthHeaders() }, body: JSON.stringify({ status })
        });
        if (ok) { showToast('Statut mis à jour'); fetchAdminMunicipalities(); fetchMunicipalities(); }
        else showToast(data.error || 'Erreur de mise à jour', 'error');
      } catch (e) { showToast('Erreur de connexion', 'error'); }
    };

    const fetchAdminAnnouncements = async () => {
      adminAnnLoading.value = true;
      try { const { ok, data } = await api('/open-panneau/api/admin/announcements', { headers: getAdminAuthHeaders() }); if (ok) adminAnnouncements.value = data.announcements || []; } catch (e) {}
      finally { adminAnnLoading.value = false; }
    };

    const adminDeleteAnnouncement = async (id) => {
      try {
        const { ok, data } = await api(`/open-panneau/api/admin/announcements/${id}`, { method: 'DELETE', headers: getAdminAuthHeaders() });
        if (ok) { showToast('Annonce supprimée'); fetchAdminAnnouncements(); fetchAnnouncements(); }
        else showToast(data.error || 'Erreur de suppression', 'error');
      } catch (e) { showToast('Erreur de connexion', 'error'); }
    };

    const statusLabel = (s) => ({ pending: 'En attente', active: 'Active', disabled: 'Désactivée' }[s] || s);
    const getTypeLabel = (type) => { const t = types.find(x => x.value === type); return t ? t.label : type; };
    const getTypeBadgeClass = (type) => ({ info: 'bg-[#6B8DB5] text-white', alert: 'bg-[var(--enbauges-terracotta)] text-white', event: 'bg-[var(--enbauges-sage)] text-white', warning: 'bg-[var(--enbauges-amber)] text-[var(--enbauges-earth)]' }[type] || 'bg-gray-500 text-white');
    const formatDate = (date, opts = {}) => new Date(date).toLocaleDateString('fr-FR', opts.short ? { day: 'numeric', month: 'short', year: 'numeric' } : { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
    const isExpired = (ann) => ann.expiresAt && new Date(ann.expiresAt) < new Date();

    onMounted(() => { fetchMunicipalities(); fetchAnnouncements(); });

    return {
      municipalities, announcements, filteredAnnouncements,
      selectedMunicipality, selectedType, loading,
      toast, dismissToast,
      confirm, confirmedAction, confirmDelete, confirmAdminDelete,
      types, aboutItems, adminFilters, adminStatCards,
      currentMunicipality, loginEmail, loginCode, showLogin, loginLoading, loginError,
      regName, regEmail, regCode, regContactName, regPhone, regInsee,
      showRegister, regLoading,
      postTitle, postContent, postType, postExpires, showPost, postLoading,
      myAnnouncements,
      isAdmin, adminMunicipalities, showAdmin,
      adminUsername, adminPassword, adminLoading, adminMunicipalityLoading,
      adminAuth, adminFilter, adminSearch, adminAuthFromLogin,
      filteredAdminMunicipalities,
      adminAnnouncements, adminAnnLoading, adminAnnFilterMun, adminAnnouncementsFiltered,
      register, login, closeLogin, logout,
      postAnnouncement, deleteAnnouncement,
      fetchAnnouncements, fetchMyAnnouncements, resetFilters,
      openAdminPanel, loginAdmin, logoutAdmin,
      fetchAdminMunicipalities, updateMunicipalityStatus,
      fetchAdminAnnouncements, adminDeleteAnnouncement,
      getTypeLabel, getTypeBadgeClass, formatDate, isExpired, statusLabel
    };
  }
}).mount('#app');
