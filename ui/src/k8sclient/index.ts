import { KubeConfig, ApiClient } from './apiclient';
import {
  Resource as ResourceType,
  ObjectMeta as ObjectMetaType,
} from './model';

export default {
  KubeConfig,
  ApiClient,
};

export type Resource = ResourceType;
export type ObjectMeta = ObjectMetaType;
