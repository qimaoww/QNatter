'use strict';
'require view';
'require poll';
'require rpc';
'require qnatter.theme as qnatterTheme';
'require ui';

var callLog = rpc.declare({
	object: 'luci.qnatter',
	method: 'log',
	params: [ 'instance', 'lines' ],
	expect: { '': { log: '' } }
});

var callClearLog = rpc.declare({
	object: 'luci.qnatter',
	method: 'clear_log',
	params: [ 'instance' ],
	expect: { ok: false }
});

var callStatus = rpc.declare({
	object: 'luci.qnatter',
	method: 'status',
	expect: { '': { instances: [] } }
});

return view.extend({
	render: function() {
		var currentInstance = 'default';
		var instanceSelect = E('select', {
			'class': 'cbi-input-select qnatter-instance-select',
			'change': function(ev) {
				currentInstance = ev.target.value || 'default';
				refreshLog();
			}
		});
		var log = E('textarea', {
			'class': 'cbi-input-textarea qnatter-log',
			'readonly': 'readonly',
			'wrap': 'off'
		});

		function updateInstances(instances) {
			var previous = currentInstance || instanceSelect.value || 'default';
			var options = [];

			(instances || []).forEach(function(item) {
				options.push(item.name || 'default');
			});

			if (!options.length)
				options.push('default');

			var found = options.some(function(item) {
				return item === previous;
			});

			currentInstance = found ? previous : options[0];
			instanceSelect.replaceChildren.apply(instanceSelect, options.map(function(item) {
				return E('option', { 'value': item }, [ item ]);
			}));
			instanceSelect.value = currentInstance;
		}

		var lastError = '';

		// 轮询失败时保留用户正在阅读的日志内容：只在错误变化时提示一次，
		// 日志本来就为空时才把原因写进文本框，避免每 5 秒刷屏。
		function reportError(err) {
			var message = (err && err.message) || String(err);

			if (message === lastError)
				return;

			lastError = message;

			if (!log.value)
				log.value = message;

			ui.addNotification(null, E('p', {}, [ message ]), 'danger');
		}

		function refreshLog() {
			return callLog(currentInstance, 300).then(function(data) {
				log.value = data.log || '';
				log.scrollTop = log.scrollHeight;
				lastError = '';
			}).catch(reportError);
		}

		function refresh() {
			return callStatus().then(function(data) {
				updateInstances(data.instances || []);
				return refreshLog();
			}).catch(reportError);
		}

		var clear = E('button', {
			'type': 'button',
			'class': 'cbi-button cbi-button-remove',
			'click': function() {
				if (clear.disabled)
					return;

				clear.disabled = true;
				clear.setAttribute('aria-busy', 'true');

				return callClearLog(currentInstance).then(function() {
					log.value = '';
					lastError = '';
					ui.addNotification(null, E('p', {}, [ _('Logs cleared') ]));
				}).catch(function(err) {
					var message = (err && err.message) || String(err);

					lastError = message;
					ui.addNotification(null, E('p', {}, [ _('Failed to clear logs') + ': ' + message ]), 'danger');
				}).finally(function() {
					clear.disabled = false;
					clear.setAttribute('aria-busy', 'false');
				});
			}
		}, [ _('Clear logs') ]);

		poll.add(refresh, 5);
		refresh();

		return qnatterTheme.annotate(E('div', { 'class': 'qnatter-page qnatter-log-page' + qnatterTheme.className() }, [
			E('link', {
				'rel': 'stylesheet',
				'href': L.resource('qnatter/qnatter.css') + '?v=1.1.0-r2'
			}),
			E('div', { 'class': 'qnatter-toolbar' }, [
				E('h2', {}, [ _('QNatter Logs') ]),
				E('div', { 'class': 'qnatter-toolbar-actions' }, [
					instanceSelect,
					clear
				])
			]),
			log
		]));
	}
});
