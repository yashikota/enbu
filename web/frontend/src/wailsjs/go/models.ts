export namespace app {
	
	export class Diff {
	    Added: string[];
	    Removed: string[];
	    Modified: string[];
	
	    static createFrom(source: any = {}) {
	        return new Diff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Added = source["Added"];
	        this.Removed = source["Removed"];
	        this.Modified = source["Modified"];
	    }
	}

}

export namespace desktop {
	
	export class RepoInfo {
	    path?: string;
	    owner?: string;
	    repo?: string;
	    initialized: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepoInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.owner = source["owner"];
	        this.repo = source["repo"];
	        this.initialized = source["initialized"];
	    }
	}
	export class AuthStatus {
	    authenticated: boolean;
	    username?: string;
	    repo?: RepoInfo;
	
	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.authenticated = source["authenticated"];
	        this.username = source["username"];
	        this.repo = this.convertValues(source["repo"], RepoInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeviceStart {
	    session_id: string;
	    user_code: string;
	    verification_uri: string;
	    // Go type: time
	    expires_at: any;
	    interval: number;
	    copied: boolean;
	    browser_opened: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeviceStart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session_id = source["session_id"];
	        this.user_code = source["user_code"];
	        this.verification_uri = source["verification_uri"];
	        this.expires_at = this.convertValues(source["expires_at"], null);
	        this.interval = source["interval"];
	        this.copied = source["copied"];
	        this.browser_opened = source["browser_opened"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DeviceStatus {
	    state: string;
	    message?: string;
	    username?: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.message = source["message"];
	        this.username = source["username"];
	    }
	}
	export class Environment {
	    name: string;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Environment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.current = source["current"];
	    }
	}
	export class HistoryEntry {
	    index: number;
	    // Go type: time
	    timestamp: any;
	    tag: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.tag = source["tag"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SecretItem {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new SecretItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class SecretsResponse {
	    environment: string;
	    secrets: SecretItem[];
	
	    static createFrom(source: any = {}) {
	        return new SecretsResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.environment = source["environment"];
	        this.secrets = this.convertValues(source["secrets"], SecretItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

