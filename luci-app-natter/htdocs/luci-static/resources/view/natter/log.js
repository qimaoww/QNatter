'use strict';
'require view';
'require poll';
'require rpc';
'require ui';

var callLog = rpc.declare({
	object: 'luci.natter',
	method: 'log',
	params: [ 'instance', 'lines' ],
	expect: { log: '' }
});

var callClearLog = rpc.declare({
	object: 'luci.natter',
	method: 'clear_log',
	params: [ 'instance' ],
	expect: { ok: false }
});

var callStatus = rpc.declare({
	object: 'luci.natter',
	method: 'status',
	expect: { instances: [] }
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

return view.extend({
	render: function() {
		var currentInstance = 'default';
		var instanceSelect = E('select', {
			'class': 'cbi-input-select natter-instance-select',
			'change': function(ev) {
				currentInstance = ev.target.value || 'default';
				refreshLog();
			}
		});
		var log = E('textarea', {
			'class': 'cbi-input-textarea natter-log',
			'readonly': 'readonly',
			'wrap': 'off'
		});

		function updateInstances(instances) {
			var previous = currentInstance || instanceSelect.value || 'default';
			var options = [];

			(instances || []).forEach(function(item) {
				options.push({
					name: item.name || 'default',
					label: item.label || item.name || 'default'
				});
			});

			if (!options.length)
				options.push({ name: 'default', label: 'default' });

			var found = options.some(function(item) {
				return item.name === previous;
			});

			currentInstance = found ? previous : options[0].name;
			instanceSelect.replaceChildren.apply(instanceSelect, options.map(function(item) {
				return E('option', { 'value': item.name }, [ item.label ]);
			}));
			instanceSelect.value = currentInstance;
		}

		function refreshLog() {
			return callLog(currentInstance, 300).then(function(data) {
				log.value = data.log || '';
				log.scrollTop = log.scrollHeight;
			}).catch(function(err) {
				log.value = err.message || String(err);
			});
		}

		function refresh() {
			return callStatus().then(function(data) {
				updateInstances(data.instances || []);
				return refreshLog();
			}).catch(function(err) {
				log.value = err.message || String(err);
			});
		}

		var clear = E('button', {
			'class': 'cbi-button cbi-button-remove',
			'click': function() {
				return callClearLog(currentInstance).then(function() {
					log.value = '';
					ui.addNotification(null, E('p', {}, [ _('Logs cleared') ]));
				});
			}
		}, [ _('Clear logs') ]);

		poll.add(refresh, 2);
		refresh();

		return E('div', { 'class': 'natter-page natter-log-page' + detectThemeClass() }, [
			E('link', {
				'rel': 'stylesheet',
				'href': L.resource('natter/natter.css')
			}),
			E('div', { 'class': 'natter-toolbar' }, [
				E('h2', {}, [ _('Natter Logs') ]),
				E('div', { 'class': 'natter-toolbar-actions' }, [
					instanceSelect,
					clear
				])
			]),
			log
		]);
	}
});
