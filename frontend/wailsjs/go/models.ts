export namespace main {
	
	export class DocType {
	    key: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new DocType(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	    }
	}
	export class ImageInfo {
	    width: number;
	    height: number;
	    base64: string;
	    filePath: string;
	
	    static createFrom(source: any = {}) {
	        return new ImageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.base64 = source["base64"];
	        this.filePath = source["filePath"];
	    }
	}

}

