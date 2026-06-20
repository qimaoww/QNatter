'use strict';
'require view';
'require poll';
'require rpc';

var callStatus = rpc.declare({
	object: 'luci.natter',
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
		return ' natter-theme-aurora';

	if (/luci-theme-argon|theme-argon|argon/i.test(text))
		return ' natter-theme-argon';

	return '';
}

function renderCard(item) {
	var route = item.outer_ip
		? '%s://%s:%s'.format(item.protocol || 'tcp', item.outer_ip, item.outer_port || '')
		: _('Waiting for mapping');
	var inner = item.inner_ip
		? '%s:%s'.format(item.inner_ip, item.inner_port || '')
		: (item.bind_value || item.network || '');

	return E('section', { 'class': 'natter-card' }, [
		E('div', { 'class': 'natter-card-head' }, [
			E('h3', {}, [ item.name || '-' ]),
			E('span', { 'class': 'natter-pill ' + (item.running ? 'is-running' : 'is-stopped') },
				[ item.running ? _('RUNNING') : _('NOT RUNNING') ])
		]),
		E('dl', {}, [
			E('dt', {}, [ _('Public address') ]), E('dd', {}, [ route ]),
			E('dt', {}, [ _('Internal address') ]), E('dd', {}, [ inner ]),
			E('dt', {}, [ _('WAN network') ]), E('dd', {}, [ item.network || 'wan' ]),
			E('dt', {}, [ _('qBittorrent') ]), E('dd', {}, [ item.qbittorrent_enabled ? _('Enabled') : _('Disabled') ]),
			E('dt', {}, [ _('Updated') ]), E('dd', {}, [ item.updated_at || '-' ]),
			E('dt', {}, [ _('Message') ]), E('dd', {}, [ item.message || '-' ])
		])
	]);
}

return view.extend({
	render: function() {
		var grid = E('div', { 'class': 'natter-grid' }, [
			E('div', { 'class': 'natter-empty' }, [ _('Collecting data...') ])
		]);
		var root = E('div', { 'class': 'natter-page' + detectThemeClass() }, [
			E('link', {
				'rel': 'stylesheet',
				'href': L.resource('natter/natter.css')
			}),
			E('div', { 'class': 'natter-toolbar' }, [
				E('h2', {}, [ _('Natter Status') ])
			]),
			grid
		]);

		function refresh() {
			return callStatus().then(function(data) {
				if (!data.instances || !data.instances.length) {
					grid.replaceChildren(E('div', { 'class': 'natter-empty' }, [ _('No instances configured.') ]));
					return;
				}

				grid.replaceChildren.apply(grid, data.instances.map(renderCard));
			}).catch(function(err) {
				grid.replaceChildren(E('div', { 'class': 'natter-empty' }, [ err.message || String(err) ]));
			});
		}

		poll.add(refresh, 3);
		refresh();

		return root;
	}
});
