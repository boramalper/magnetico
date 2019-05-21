(function (root, factory) {
    if (typeof define == 'function' && define.amd) {
        define( factory );
    } else if (typeof module === 'object' && module.exports) {
        module.exports = factory();
    } else {
        root.VanillaTree = factory();
    }
}(this, function () {
	"use strict";
	// Look at the Balalaika https://github.com/finom/balalaika
	var $=function(n,e,k,h,p,m,l,b,d,g,f,c){c=function(a,b){return new c.i(a,b)};c.i=function(a,d){k.push.apply(this,a?a.nodeType||a==n?[a]:""+a===a?/</.test(a)?((b=e.createElement(d||"q")).innerHTML=a,b.children):(d&&c(d)[0]||e).querySelectorAll(a):/f/.test(typeof a)?/c/.test(e.readyState)?a():c(e).on("DOMContentLoaded",a):a:k)};c.i[f="prototype"]=(c.extend=function(a){g=arguments;for(b=1;b<g.length;b++)if(f=g[b])for(d in f)a[d]=f[d];return a})(c.fn=c[f]=k,{on:function(a,d){a=a.split(h);this.map(function(c){(h[b=a[0]+(c.b$=c.b$||++p)]=h[b]||[]).push([d,a[1]]);c["add"+m](a[0],d)});return this},off:function(a,c){a=a.split(h);f="remove"+m;this.map(function(e){if(b=(g=h[a[0]+e.b$])&&g.length)for(;d=g[--b];)c&&c!=d[0]||a[1]&&a[1]!=d[1]||(e[f](a[0],d[0]),g.splice(b,1));else!a[1]&&e[f](a[0],c)});return this},is:function(a){d=(b=this[0])&&(b.matches||b["webkit"+l]||b["moz"+l]||b["ms"+l]);return!!d&&d.call(b,a)}});return c}(window,document,[],/\.(.+)/,0,"EventListener","MatchesSelector");

	var create = function( tagName, props ) {
			return $.extend( document.createElement( tagName ), props );
		},
		Tree = function( s, options ) {
			var _this = this,
				container = _this.container = $( s )[ 0 ],
				tree = _this.tree = container.appendChild( create( 'ul', {
					className: 'vtree'
				}) );

			_this.placeholder = options && options.placeholder;
			_this._placeholder();
			_this.leafs = {};
			tree.addEventListener( 'click', function( evt ) {
				if( $( evt.target ).is( '.vtree-leaf-label' ) ) {
					_this.select( evt.target.parentNode.getAttribute('data-vtree-id') );
				} else if( $( evt.target ).is( '.vtree-toggle' ) ) {
					_this.toggle( evt.target.parentNode.getAttribute('data-vtree-id') );
				}
			});

			if( options && options.contextmenu ) {
				tree.addEventListener( 'contextmenu', function( evt ) {
					var menu;
					$( '.vtree-contextmenu' ).forEach( function( menu ) {
						menu.parentNode.removeChild( menu );
					});
					if( $( evt.target ).is( '.vtree-leaf-label' ) ) {
						evt.preventDefault();
						evt.stopPropagation();
						menu = create( 'menu', {
							className: 'vtree-contextmenu'
						});

            var rect = evt.target.getBoundingClientRect();
            $.extend(menu.style, {
                top: (evt.target.offsetTop + rect.height).toString() + "px",
                left: evt.target.offsetLeft.toString() + "px",
                display: 'block'
            });

						options.contextmenu.forEach( function( item ) {
							menu.appendChild( create( 'li', {
								className: 'vtree-contextmenu-item',
								innerHTML: item.label
							}) ).addEventListener( 'click', item.action.bind( item, evt.target.parentNode.getAttribute('data-vtree-id') ) );
						});

						evt.target.parentNode.appendChild( menu );
					}
				});

				document.addEventListener( 'click', function( evt ) {
          if(evt.button === 2) return;
					$( '.vtree-contextmenu' ).forEach( function( menu ) {
						menu.parentNode.removeChild( menu );
					});
				});
			}
		};

	Tree.prototype = {
		constructor: Tree,
		_dispatch: function( name, id ) {
			var event;
			try {
				event = new CustomEvent( 'vtree-' + name, {
					bubbles: true,
					cancelable: true,
					detail: {
						id: id
					}
				});
			} catch(e) {
				event = document.createEvent( 'CustomEvent' );
				event.initCustomEvent( 'vtree-' + name, true, true, { id: id });
			}
			( this.getLeaf( id, true ) || this.tree )
				.dispatchEvent( event );
			return this;
		},
		_placeholder: function() {
			var p;
			if( !this.tree.children.length && this.placeholder ) {
				this.tree.innerHTML = '<li class="vtree-placeholder">' + this.placeholder + '</li>'
			} else if( p = this.tree.querySelector( '.vtree-placeholder' ) ) {
				this.tree.removeChild( p );
			}
			return this;
		},
		getLeaf: function( id, notThrow ) {
			var leaf = $( '[data-vtree-id="' + id + '"]', this.tree )[ 0 ];
			if( !notThrow && !leaf ) throw Error( 'No VanillaTree leaf with id "' + id + '"' )
			return leaf;
		},
		getChildList: function( id ) {
			var list,
				parent;
			if( id ) {
				parent = this.getLeaf( id );
				if( !( list = $( 'ul', parent )[ 0 ] ) ) {
					list = parent.appendChild( create( 'ul', {
						className: 'vtree-subtree'
					}) );
				}
			} else {
				list = this.tree;
			}

			return list;
		},
		add: function( options ) {
			// see https://github.com/finom/vanillatree/issues/8
			if(this.getLeaf( options.id,true )){return;}// don't add leaves that already exist
			var id,
				leaf = create( 'li', {
					className: 'vtree-leaf'
				}),
				parentList = this.getChildList( options.parent );

			leaf.setAttribute( 'data-vtree-id', id = options.id || Math.random() );

			leaf.appendChild( create( 'span', {
				className: 'vtree-toggle'
			}) );

			leaf.appendChild( create( 'a', {
				className: 'vtree-leaf-label',
				innerHTML: options.label
			}) );

			parentList.appendChild( leaf );

			if( parentList !== this.tree ) {
				parentList.parentNode.classList.add( 'vtree-has-children' );
			}

			this.leafs[ id ] = options;

			if( !options.opened ) {
				this.close( id );
			}

			if( options.selected ) {
				this.select( id );
			}

			return this._placeholder()._dispatch( 'add', id );
		},
		move: function( id, parentId ) {
			var leaf = this.getLeaf( id ),
				oldParent = leaf.parentNode,
				newParent = this.getLeaf( parentId, true );

			if( newParent ) {
				newParent.classList.add( 'vtree-has-children' );
			}

			this.getChildList( parentId ).appendChild( leaf );
			oldParent.parentNode.classList.toggle( 'vtree-has-children', !!oldParent.children.length );

			return this._dispatch( 'move', id );
		},
		remove: function( id ) {
			var leaf = this.getLeaf( id ),
				oldParent = leaf.parentNode;
			oldParent.removeChild( leaf );
			oldParent.parentNode.classList.toggle( 'vtree-has-children', !!oldParent.children.length );

			return this._placeholder()._dispatch( 'remove', id );
		},
		open: function( id ) {
			this.getLeaf( id ).classList.remove( 'closed' );
			return this._dispatch( 'open', id );
		},
		close: function( id ) {
			this.getLeaf( id ).classList.add( 'closed' );
			return this._dispatch( 'close', id );
		},
		toggle: function( id ) {
			return this[ this.getLeaf( id ).classList.contains( 'closed' ) ? 'open' : 'close' ]( id );
		},
		select: function( id ) {
			var leaf = this.getLeaf( id );

			if( !leaf.classList.contains( 'vtree-selected' ) ) {
				$( 'li.vtree-leaf', this.tree ).forEach( function( leaf ) {
					leaf.classList.remove( 'vtree-selected' );
				});

				leaf.classList.add( 'vtree-selected' );
				this._dispatch( 'select', id );
			}

			return this;
		}
	};

	return Tree;
	// Look at the Balalaika https://github.com/finom/balalaika
}));
