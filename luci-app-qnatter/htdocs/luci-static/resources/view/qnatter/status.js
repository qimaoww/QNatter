'use strict';
'require view';
'require poll';
'require rpc';

var callStatus = rpc.declare({
	object: 'luci.qnatter',
	method: 'status',
	expect: { '': { instances: [] } }
});

function detectThemeClass() {
	var text = [
		document.documentElement.className || '',
		document.body ? document.body.className || '' : '',
		Array.prototype.map.call(document.querySelectorAll('link[href]'), function(link) {
			return link.getAttribute('href') || '';
		}).join(' ')
	].join(' ');

	if (/luci-theme-aurora|theme-aurora|aurora/i.test(text))
		return ' qnatter-theme-aurora';

	if (/luci-theme-argon|theme-argon|argon/i.test(text))
		return ' qnatter-theme-argon';

	return '';
}

var callReloadInstance = rpc.declare({
	object: 'luci.qnatter',
	method: 'reload_instance',
	params: [ 'instance' ],
	expect: { '': { ok: true } }
});

function reloadInstance(name, btn) {
	if (btn) {
		btn.disabled = true;
		btn.innerHTML = _('Reloading…');
	}
	return callReloadInstance(name).then(function() {
		return new Promise(function(resolve) { setTimeout(resolve, 2000); });
	}).then(function() {
		return callStatus();
	}).catch(function(err) {
		alert(err.message || String(err));
	}).finally(function() {
		if (btn) {
			btn.disabled = false;
			btn.innerHTML = _('Reload');
		}
	});
}

function renderCard(item) {
	var route = item.outer_ip
		? '%s:%s'.format(item.outer_ip, item.outer_port || '')
		: _('Waiting for mapping');
	var inner = item.inner_ip
		? '%s:%s'.format(item.inner_ip, item.inner_port || '')
		: (item.bind_value || item.network || '');
	var protocol = (item.protocol || 'tcp').toString().toUpperCase();

	var reloadBtn = E('button', {
		'class': 'btn cbi-button cbi-button-action',
		'style': 'margin-left:6px;padding:2px 8px;font-size:11px',
		'click': function(ev) { reloadInstance(item.instance || item.name, this); }
	}, [ _('Reload') ]);

	return E('section', { 'class': 'qnatter-card' }, [
		E('div', { 'class': 'qnatter-card-head' }, [
			E('h3', { 'style': 'display:flex;align-items:center;gap:4px' }, [
				item.name || '-',
				reloadBtn
			]),
			E('span', { 'class': 'qnatter-pill ' + (item.running ? 'is-running' : 'is-stopped') },
				[ item.running ? _('RUNNING') : _('NOT RUNNING') ])
		]),
		E('dl', {}, [
			E('dt', {}, [ _('Public address') ]), E('dd', {}, [ route ]),
			E('dt', {}, [ _('Internal address') ]), E('dd', {}, [ inner ]),
			E('dt', {}, [ _('Network protocol') ]), E('dd', {}, [ protocol ]),
			E('dt', {}, [ _('WAN network') ]), E('dd', {}, [ item.network || 'wan' ]),
			E('dt', {}, [ _('qBittorrent') ]), E('dd', {}, [ item.qbittorrent_enabled ? _('Enabled') : _('Disabled') ]),
			E('dt', {}, [ _('Updated') ]), E('dd', {}, [ item.updated_at || '-' ]),
			E('dt', {}, [ _('Message') ]), E('dd', {}, [ item.message || '-' ])
		])
	]);
}

return view.extend({
	render: function() {
		var grid = E('div', { 'class': 'qnatter-grid' }, [
			E('div', { 'class': 'qnatter-empty' }, [ _('Collecting data...') ])
		]);
		var root = E('div', { 'class': 'qnatter-page' + detectThemeClass() }, [
			E('link', {
				'rel': 'stylesheet',
				'href': L.resource('qnatter/qnatter.css')
			}),
			E('div', { 'class': 'qnatter-toolbar' }, [
				E('h2', {}, [ _('QNatter Status') ])
			]),
			grid
		]);

		function refresh() {
			return callStatus().then(function(data) {
				if (!data.instances || !data.instances.length) {
					grid.replaceChildren(E('div', { 'class': 'qnatter-empty' }, [ _('No instances configured.') ]));
					return;
				}

				grid.replaceChildren.apply(grid, data.instances.map(renderCard));
			}).catch(function(err) {
				grid.replaceChildren(E('div', { 'class': 'qnatter-empty' }, [ err.message || String(err) ]));
			});
		}

		poll.add(refresh, 3);
		refresh();

		return root;
	}
});
