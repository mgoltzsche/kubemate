export interface Resource {
  readonly metadata?: ObjectMeta;
}

export interface ResourceList<T> {
  readonly metadata: ObjectMeta;
  readonly items: Array<T>;
}

export interface ObjectMeta {
  readonly name?: string;
  readonly namespace?: string;
  readonly resourceVersion?: string;
}

/*export interface CustomResource extends Resource {
  readonly spec: any;
  readonly status: any;
}*/
