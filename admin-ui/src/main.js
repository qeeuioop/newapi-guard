import { createApp } from "vue";
import {
  create,
  NAlert,
  NButton,
  NConfigProvider,
  NDivider,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPagination,
  NSelect,
  NSpin,
  NSwitch,
  NTabPane,
  NTable,
  NTabs,
  NTag
} from "naive-ui";
import App from "./App.vue";
import "./style.css";

const naive = create({
  components: [
    NAlert,
    NButton,
    NConfigProvider,
    NDivider,
    NEmpty,
    NForm,
    NFormItem,
    NInput,
    NInputNumber,
    NModal,
    NPagination,
    NSelect,
    NSpin,
    NSwitch,
    NTable,
    NTabPane,
    NTabs,
    NTag
  ]
});

createApp(App).use(naive).mount("#app");
