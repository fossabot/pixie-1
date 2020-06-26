import * as React from 'react';
import { WidgetDisplay } from 'containers/live/vis';

import { Network, data as visData} from 'vis-network/standalone';
import { createStyles, makeStyles } from '@material-ui/core/styles';
import { toEntityPathname, toSingleEntityPage } from 'containers/live/utils/live-view-params';
import ClusterContext from 'common/cluster-context';
import { SemanticType } from 'types/generated/vizier_pb';
import Button from '@material-ui/core/Button';
import { useHistory } from 'react-router-dom';
import {
  getColorForErrorRate,
  getColorForLatency,
  GRAPH_OPTIONS as graphOpts,
  LABEL_OPTIONS as labelOpts,
} from './graph-options';
import { RequestGraph, RequestGraphParser, Edge } from './request-graph-parser';

import * as svcSVG from './svc.svg';
import { formatFloat64Data } from 'utils/format-data';

export interface RequestGraphDisplay extends WidgetDisplay {
  readonly requestorPodColumn: string;
  readonly responderPodColumn: string;
  readonly requestorServiceColumn: string;
  readonly responderServiceColumn: string;
  readonly p50Column: string;
  readonly p90Column: string;
  readonly p99Column: string;
  readonly errorRateColumn: string;
  readonly requestsPerSecondColumn: string;
  readonly inboundBytesPerSecondColumn: string;
  readonly outboundBytesPerSecondColumn: string;
  readonly totalRequestCountColumn: string;
}

interface RequestGraphProps {
  display: RequestGraphDisplay;
  data: any[];
}

const useStyles = makeStyles(() =>
  createStyles({
    root: {
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'flex-end',
      border: '1px solid #161616',
      '&.focus': {
        border: '1px solid #353738',
      }
    },
    container: {
      width: '100%',
      height: '95%',
      '& > .vis-active': {
        boxShadow: 'none',
      }
    },
  }),
);


export const RequestGraphWidget = (props: RequestGraphProps) => {
  const { selectedClusterName } = React.useContext(ClusterContext);
  const history = useHistory();

  const ref = React.useRef<HTMLDivElement>()
  const data = props.data; //parseDOTNetwork(dot);
  const display = props.display;

  const [network, setNetwork] = React.useState<Network>(null);
  const [graph, setGraph] = React.useState<RequestGraph>(null);

  const [clusteredMode, setClusteredMode] = React.useState<boolean>(false);
  const [hierarchyEnabled, setHierarchyEnabled] = React.useState<boolean>(false);
  const [colorByLatency, setColorByLatency] = React.useState<boolean>(false);
  const [focused, setFocused] = React.useState<boolean>(false);

  /**
   * Toggle the hier/non-hier clustering mode.
   */
  const toggleMode = React.useCallback(() => setClusteredMode((clustered) => {
    const nextState = !clustered;
    if (!network || !graph) {
      return nextState
    }
    network.setData({
      nodes: graph.nodes,
      edges: graph.edges,
    });
    if (nextState) {
      // Clustered.
      graph.services.forEach((svc) => {
        const clusterOptionsByData = {
          joinCondition: function(childOptions) {
            return childOptions.service === svc;
          },
          clusterNodeProperties: {
            shape: 'image',
            image: {
              selected: svcSVG,
              unselected: svcSVG,
            },
            allowSingleNodeCluster: true,
            label: svc,
            scaling: labelOpts,
          },
          processProperties: function(clusterOptions,
                                      childNodes) {
            let totalValue = 0;
            childNodes.forEach((node) => {
              totalValue += node.value;
            })
            clusterOptions.value = totalValue;
            clusterOptions.id = svc;
            if (svc === '') {
              clusterOptions.hidden = true;
              clusterOptions.physics = false;
            }
            return clusterOptions;
          },
        };
        network.cluster(clusterOptionsByData);
      });
    }

    return nextState;
  }), [network, graph]);

  /**
   * This is used to toggle the hierarchical state of graph.
   */
  const toggleHierarchy = React.useCallback(() => setHierarchyEnabled((enabled) => {
    const hierEnabled = !enabled;
    if (!network) {
      return hierEnabled;
    }
    if (hierEnabled) {
      const opts = {
        ...graphOpts,
        layout: {
          ...graphOpts.layout,
          hierarchical: {
            enabled: true,
            levelSeparation: 200,
            direction: 'LR',
            sortMethod: 'directed',
          },
        }
      };
      network.setOptions(opts)
    } else {
      const opts = {...graphOpts,
      layout: {
        ...graphOpts.layout,
        hierarchical: {
          enabled: false,
        },
      }};

      network.setOptions(opts)
    }
    return hierEnabled;
  }), [network]);

  const toggleColor = React.useCallback(() => setColorByLatency((enabled) => {
    const colorByLatency = !enabled;
    graph && graph.edges.forEach((edge: Edge) => {
      graph.edges.update({...edge,
        color: colorByLatency ? getColorForLatency(edge.p99) : getColorForErrorRate(edge.errorRate),
      });
    });
    return colorByLatency;
  }), [graph]);

  const toggleFocus = React.useCallback(() => setFocused((enabled) => !enabled), []);

  const doubleClickCallback = React.useCallback((params?: any) => {
    if (params.nodes.length > 0) {
      const semType = !clusteredMode ?  SemanticType.ST_POD_NAME : SemanticType.ST_SERVICE_NAME;
      const page = toSingleEntityPage(params.nodes[0], semType, selectedClusterName);
      const pathname = toEntityPathname(page);
      history.push(pathname);
    }
  }, [clusteredMode, graph]);

  // Load the graph.
  React.useEffect(() => {
    const p = new RequestGraphParser(data, display)
    const nodeDS = new visData.DataSet();
    nodeDS.add(p.getEntities());
    const edgeDS = new visData.DataSet();
    edgeDS.add(p.getEdges());
    setGraph({
      nodes: nodeDS,
      edges: edgeDS,
      services: p.getServiceList(),
    });
  }, [data, display]);

  // Load the data.
  React.useEffect(()=> {
    if (ref && graph) {
      // Hydrate the data.
      graph.edges.forEach((edge: Edge) => {
        const bps = edge.inboundBPS + edge.outputBPS;
        const title = `${formatFloat64Data(bps)} B/s <br />
                       ${formatFloat64Data(edge.rps)} req/s <br >
                       Error: ${formatFloat64Data(edge.errorRate) + '%'} <br>
                       p50: ${formatFloat64Data(edge.p50)} ms <br>
                       p99: ${formatFloat64Data(edge.p99)} ms`;

        const color = colorByLatency ? getColorForLatency(edge.p99) : getColorForErrorRate(edge.errorRate);
        const value = bps;
        graph.edges.update({
          ...edge,
          title,
          color,
          value,
        })
      });

      const data = {
        nodes: graph.nodes,
        edges: graph.edges,
      }
      const network = new Network(ref.current, data, graphOpts);
      network.on('doubleClick', doubleClickCallback)
      setNetwork(network);
    }
  }, [graph, ref, colorByLatency])

  const classes = useStyles();
  return (
    <div className={classes.root + ' ' + (focused ? 'focus' : '')} onFocus={toggleFocus} onBlur={toggleFocus}>
      <div  className={classes.container} ref={ref} />
      <div>
        <Button
          size='small'
          onClick={toggleColor}>{colorByLatency ? 'Color by error rate' : 'Color by latency'}
        </Button>
        <Button
          size='small'
          onClick={toggleHierarchy}>{hierarchyEnabled ? 'Disable hierarchy' : 'Enable hierarchy'}
        </Button>
        <Button
          size='small'
          onClick={toggleMode}>{clusteredMode ? 'Disable clustering' : 'Cluster by service'}
        </Button>
      </div>
    </div>);
};