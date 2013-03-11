var conn, sendMessage;

$(function() {
	var frames = $('iframe');
	var osd = $('#osd');

	frames.on('load', function() {
		console.log('loaded, will swap');
		if (this.src) {
			swap();
		}
	});

	function osdText(text) {
		osd.html(text).show();
		osd.fitText(1.2, {maxFontSize: '200px', minFontSize: '28px'});
	}

	function connect() {
		var reconnectTimer;
		var host = 'ws://' + window.location.host + window.location.pathname.replace(/\/[^\/]+$/, '/') + 'pusher';

		conn = new WebSocket(host);

		conn.onclose = function(evt) {
			console.log("onclose", evt);

			// Set a reconnect timer...
			if (reconnectTimer) {
				clearTimeout(reconnectTimer);
			}

			reconnectTimer = setTimeout(connect, 3000);
		};

		conn.onopen = function(evt) {
			console.log("onopen", evt);
		};

		conn.onerror = function(evt) {
			console.log("onerror", evt);
		};

		conn.onmessage = function(evt) {
			console.log("onmessage", evt);

			var message = JSON.parse(evt.data);

			if (message.type == "url") {
				loadURL(message.payload);
				osdText('connected');
			} else if (message.type == "reload") {
				var currentLocation = window.location;
				window.location = currentLocation;
			}
		};
	}

	sendMessage = function(type, payload) {
		console.log("Sending msg", type, payload);
		conn.send(JSON.stringify({ "type": type, "payload": payload }));
	};

	function currentFrame() {
		return $('iframe.current');
	}

	function nextFrame() {
		return $('iframe.next');
	}

	function swap() {
		var current = currentFrame();
		var next = nextFrame();

		next.removeClass('loading');
		current.removeClass('current'); current.addClass('next');
		next.removeClass('next'); next.addClass('current');
		console.log('swapped');
	}

	function loadURL(url) {
		var next = nextFrame();
		next.addClass('loading');
		next.attr('src', url);
	}

	connect();
});
