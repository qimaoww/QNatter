'use strict';
'require view';
'require rpc';

var callStatus = rpc.declare({
	object: 'luci.qnatter',
	method: 'status',
	expect: { '': { instances: [] } }
});

function detectThemeClass() {
	var hints = [];
	try {
		hints.push(document.documentElement.className || '');
	} catch (e) {}
	if (document.body) {
		try { hints.push(document.body.className || ''); } catch (e) {}
		try { hints.push(getComputedStyle(document.body).backgroundColor || ''); } catch (e) {}
	}
	try {
		for (var i = 0; i < document.styleSheets.length; i++) {
			try { hints.push(document.styleSheets[i].href || ''); } catch (e) {}
		}
	} catch (e) {}
	try {
		var links = document.querySelectorAll('link[href]');
		for (var j = 0; j < links.length; j++)
			hints.push(links[j].getAttribute('href') || '');
	} catch (e) {}
	var text = hints.join(' ');

	var result = '';
	// 先检测 Argon（优先级高，防止被 Aurora 误判）
	if (/\/argon\/|luci-theme-argon|theme-argon/i.test(text)) {
		var dark = /argon\/css\/dark\.css/i.test(text) ||
			(document.body && /rgb\(30,\s*30,\s*30\)|#1e1e1e/i.test(getComputedStyle(document.body).backgroundColor || ''));
		result = ' qnatter-theme-argon' + (dark ? ' qnatter-argon-dark' : '');
	} else if (/\/aurora\/|luci-theme-aurora|theme-aurora/i.test(text)) {
		result = ' qnatter-theme-aurora';
	}

	// 诊断用：在 DOM 上暴露检测结果
	try { document.documentElement.setAttribute('data-qnatter-theme', (result || 'none').trim()); } catch (e) {}
	return result;
}

var callToggleInstance = rpc.declare({
	object: 'luci.qnatter',
	method: 'toggle_instance',
	params: [ 'instance' ],
	expect: { '': { ok: true } }
});

function toggleInstance(name, btn) {
	if (btn.disabled)
		return;

	var wasEnabled = btn.getAttribute('data-enabled') == '1';
	btn.disabled = true;
	btn.classList.add('qnatter-pill-loading');

	return callToggleInstance(name).then(function(result) {
		if (result && result.enabled !== undefined) {
			btn.setAttribute('data-enabled', result.enabled);
		}
		return new Promise(function(resolve) { setTimeout(resolve, 1000); });
	}).catch(function(err) {
		btn.setAttribute('data-enabled', wasEnabled ? '1' : '0');
		alert(err.message || String(err));
	}).finally(function() {
		btn.disabled = false;
		btn.classList.remove('qnatter-pill-loading');
	});
}

function setText(node, text) {
	text = text == null ? '' : String(text);
	if (node.textContent !== text)
		node.textContent = text;
}

function itemKey(item) {
	return item.name || item.instance || 'default';
}

function itemRoute(item) {
	return item.outer_ip
		? '%s:%s'.format(item.outer_ip, item.outer_port || '')
		: _('Waiting for mapping');
}

function itemInner(item) {
	return item.inner_ip
		? '%s:%s'.format(item.inner_ip, item.inner_port || '')
		: (item.bind_value || item.network || '');
}

function createCard(item, fieldByName) {
	var name = itemKey(item);
	var fields = {};

	fields.name = E('span', {}, [ name || '-' ]);
	fields.running = E('button', {
		'class': 'qnatter-pill qnatter-pill-clickable',
		'data-enabled': item.enabled ? '1' : '0',
		'click': function(ev) { toggleInstance(name, this); }
	}, []);
	fields.route = E('dd', {}, []);
	fields.inner = E('dd', {}, []);
	fields.protocol = E('dd', {}, []);
	fields.network = E('dd', {}, []);
	fields.qbittorrent = E('dd', {}, []);
	fields.updated_at = E('dd', {}, []);
	fields.message = E('dd', {}, []);
	fieldByName[name] = fields;

	return E('section', { 'class': 'qnatter-card', 'data-instance': name }, [
		E('div', { 'class': 'qnatter-card-head' }, [
			E('h3', {}, [ fields.name ]),
			fields.running
		]),
		E('dl', {}, [
			E('dt', {}, [ _('Public address') ]), fields.route,
			E('dt', {}, [ _('Internal address') ]), fields.inner,
			E('dt', {}, [ _('Network protocol') ]), fields.protocol,
			E('dt', {}, [ _('WAN network') ]), fields.network,
			E('dt', {}, [ _('qBittorrent') ]), fields.qbittorrent,
			E('dt', {}, [ _('Updated') ]), fields.updated_at,
			E('dt', {}, [ _('Message') ]), fields.message
		])
	]);
}

function updateCard(item, fieldByName) {
	var name = itemKey(item);
	var fields = fieldByName[name];
	var route = itemRoute(item);
	var inner = itemInner(item);
	var protocol = (item.protocol || 'tcp').toString().toUpperCase();

	if (!fields)
		return;

	setText(fields.name, name || '-');
	if (fields.running && !fields.running.disabled) {
		fields.running.setAttribute('data-enabled', item.enabled ? '1' : '0');
		fields.running.className = 'qnatter-pill qnatter-pill-clickable ' + (item.running ? 'is-running' : 'is-stopped');
		setText(fields.running, item.running ? _('RUNNING') : _('NOT RUNNING'));
	}
	setText(fields.route, route);
	setText(fields.inner, inner);
	setText(fields.protocol, protocol);
	setText(fields.network, item.network || 'wan');
	setText(fields.qbittorrent, item.qbittorrent_enabled ? _('Enabled') : _('Disabled'));
	setText(fields.updated_at, item.updated_at || '-');
	setText(fields.message, item.message || '-');
}

return view.extend({
	render: function() {
		var cardByName = {};
		var fieldByName = {};
		var refreshInFlight = false;
		var refreshTimer = null;
		var grid = E('div', { 'class': 'qnatter-grid' }, [
			E('div', { 'class': 'qnatter-empty' }, [ _('Collecting data...') ])
		]);
		var root = E('div', { 'class': 'qnatter-page' + detectThemeClass() }, [
			E('link', {
				'rel': 'stylesheet',
				'href': L.resource('qnatter/qnatter.css') + '?v=1.0.0-r39'
			}),
			E('div', { 'class': 'qnatter-toolbar' }, [
				E('h2', {}, [ _('QNatter Status') ])
			]),
			grid
		]);

		function renderInstances(instances) {
			var present = {};

			if (!instances.length) {
				cardByName = {};
				fieldByName = {};
				grid.replaceChildren(E('div', { 'class': 'qnatter-empty' }, [ _('No instances configured.') ]));
				return;
			}

			if (grid.firstElementChild && grid.firstElementChild.className === 'qnatter-empty')
				grid.replaceChildren();

			for (var i = 0; i < instances.length; i++) {
				var item = instances[i];
				var name = itemKey(item);
				present[name] = true;

				if (!cardByName[name]) {
					cardByName[name] = createCard(item, fieldByName);
				}

				grid.appendChild(cardByName[name]);
				updateCard(item, fieldByName);
			}

			Object.keys(cardByName).forEach(function(name) {
				if (!present[name]) {
					cardByName[name].remove();
					delete cardByName[name];
					delete fieldByName[name];
				}
			});
		}

		function refresh() {
			if (refreshInFlight)
				return Promise.resolve();

			refreshInFlight = true;
			return callStatus().then(function(data) {
				renderInstances(data.instances || []);
			}).catch(function(err) {
				cardByName = {};
				fieldByName = {};
				grid.replaceChildren(E('div', { 'class': 'qnatter-empty' }, [ err.message || String(err) ]));
			}).finally(function() {
				refreshInFlight = false;
			});
		}

		function scheduleRefresh(delay) {
			window.clearTimeout(refreshTimer);
			refreshTimer = window.setTimeout(function() {
				refresh().finally(function() {
					scheduleRefresh(document.hidden ? 10000 : 1000);
				});
			}, delay);
		}

		document.addEventListener('visibilitychange', function() {
			if (!document.hidden) {
				window.clearTimeout(refreshTimer);
				refresh().finally(function() { scheduleRefresh(1000); });
			}
		});

		refresh();
		scheduleRefresh(1000);

		return root;
	}
});
